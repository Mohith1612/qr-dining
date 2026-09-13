import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  site: "https://qrdining.app",
  output: "static",
  trailingSlash: "never",
  integrations: [sitemap()],
  vite: { plugins: [tailwindcss()] },
  redirects: {
    "/contact": { status: 301, destination: "/demo" }
  }
});
