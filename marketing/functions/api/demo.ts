interface Env {
  RESEND_API_KEY: string;
  DEMO_TO_EMAIL?: string;
  DEMO_FROM_EMAIL?: string;
  RATE_LIMIT?: KVNamespace;
}

type DemoPayload = {
  name?: string;
  restaurant?: string;
  city?: string;
  phone?: string;
  email?: string;
  restaurantType?: string;
  message?: string;
  website?: string;
};

const json = (data: unknown, status = 200) =>
  new Response(JSON.stringify(data), {
    status,
    headers: { "content-type": "application/json; charset=utf-8" },
  });

const clean = (value: unknown, max = 300) =>
  typeof value === "string" ? value.trim().slice(0, max) : "";

const escapeHtml = (value: string) =>
  value.replace(/[&<>"']/g, character => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;",
  })[character] ?? character);

export const onRequestPost: PagesFunction<Env> = async context => {
  const contentType = context.request.headers.get("content-type") || "";
  if (!contentType.includes("application/json")) return json({ error: "Expected JSON." }, 415);

  let body: DemoPayload;
  try { body = await context.request.json() as DemoPayload; }
  catch { return json({ error: "Invalid request." }, 400); }

  // Honeypot submissions receive a normal response so bots do not learn the trap.
  if (clean(body.website)) return json({ ok: true });

  const payload = {
    name: clean(body.name, 100),
    restaurant: clean(body.restaurant, 160),
    city: clean(body.city, 100),
    phone: clean(body.phone, 30),
    email: clean(body.email, 180).toLowerCase(),
    restaurantType: clean(body.restaurantType, 60),
    message: clean(body.message, 2000),
  };
  if (
    payload.name.length < 2 ||
    payload.restaurant.length < 2 ||
    payload.city.length < 2 ||
    payload.phone.replace(/\D/g, "").length < 10 ||
    !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(payload.email) ||
    !payload.restaurantType
  ) return json({ error: "Please check the required fields." }, 400);

  const ip = context.request.headers.get("CF-Connecting-IP") || "unknown";
  if (context.env.RATE_LIMIT) {
    const bucket = Math.floor(Date.now() / 3_600_000);
    const key = `demo:${ip}:${bucket}`;
    const count = Number(await context.env.RATE_LIMIT.get(key) || "0");
    if (count >= 5) return json({ error: "Too many requests. Please try again later." }, 429);
    await context.env.RATE_LIMIT.put(key, String(count + 1), { expirationTtl: 7200 });
  }

  if (!context.env.RESEND_API_KEY) return json({ error: "Email delivery is not configured." }, 503);
  const safe = Object.fromEntries(Object.entries(payload).map(([key, value]) => [key, escapeHtml(value)]));
  const response = await fetch("https://api.resend.com/emails", {
    method: "POST",
    headers: {
      authorization: `Bearer ${context.env.RESEND_API_KEY}`,
      "content-type": "application/json",
    },
    body: JSON.stringify({
      from: context.env.DEMO_FROM_EMAIL || "QR Dining <website@qrdining.app>",
      to: [context.env.DEMO_TO_EMAIL || "hello@qrdining.app"],
      reply_to: payload.email,
      subject: `Demo request — ${payload.restaurant}, ${payload.city}`,
      html: `<h2>New QR Dining demo request</h2>
        <p><strong>Name:</strong> ${safe.name}</p>
        <p><strong>Restaurant:</strong> ${safe.restaurant}</p>
        <p><strong>City:</strong> ${safe.city}</p>
        <p><strong>Phone:</strong> ${safe.phone}</p>
        <p><strong>Email:</strong> ${safe.email}</p>
        <p><strong>Type:</strong> ${safe.restaurantType}</p>
        <p><strong>Message:</strong><br>${safe.message || "—"}</p>`,
    }),
  });
  if (!response.ok) return json({ error: "Email delivery failed." }, 502);
  return json({ ok: true }, 201);
};

export const onRequest: PagesFunction<Env> = async context => {
  if (context.request.method !== "POST") return json({ error: "Method not allowed." }, 405);
  return onRequestPost(context);
};
