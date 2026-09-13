export const site = {
  name: "QR Dining",
  url: "https://qrdining.app",
  email: "hello@qrdining.app",
  appUrl: import.meta.env.PUBLIC_APP_URL || "https://app.qrdining.app",
};

export const plans = [
  {
    name: "Free",
    price: 0,
    summary: "For a small room trying table ordering.",
    features: {
      branches: "1 branch",
      tables: "Up to 10",
      operationalAnalytics: false,
      multiBranchAnalytics: false,
      loyalty: false,
      customTheming: false,
    },
  },
  {
    name: "Standard",
    price: 1499,
    summary: "For a busy restaurant running daily service.",
    popular: true,
    features: {
      branches: "Up to 3",
      tables: "Unlimited",
      operationalAnalytics: true,
      multiBranchAnalytics: false,
      loyalty: true,
      customTheming: false,
    },
  },
  {
    name: "Premium",
    price: 2999,
    summary: "For groups that need every branch in one place.",
    features: {
      branches: "Unlimited",
      tables: "Unlimited",
      operationalAnalytics: true,
      multiBranchAnalytics: true,
      loyalty: true,
      customTheming: true,
    },
  },
] as const;

export const faqs = [
  {
    question: "Do guests need an app?",
    answer: "No. They scan the table QR and join in any modern phone browser. There is no download and no guest account required.",
  },
  {
    question: "What hardware do we need?",
    answer: "Any phone, tablet, or computer with a browser can run the staff boards. We provide print-ready QR cards; you can print them locally or use your existing printer.",
  },
  {
    question: "How do payments work?",
    answer: "Guests can choose UPI, cash, or card at the table. Your staff verifies and settles the payment, and QR Dining records it against the bill. Direct online payments are rolling out separately.",
  },
  {
    question: "What about GST?",
    answer: "Bills are GST-aware, with tax amounts carried through the bill and settlement record. We configure your restaurant's tax setup during onboarding.",
  },
  {
    question: "Can we use our own branding?",
    answer: "Every plan includes polished guest themes. Premium adds custom theming, and the QR collateral studio applies your restaurant name and branding to print-ready cards.",
  },
  {
    question: "How long does setup take?",
    answer: "For a pilot restaurant, we prepare the menu, tables, staff access, and QR cards with you. Timing depends mostly on the size and readiness of your menu.",
  },
  {
    question: "What if the internet drops?",
    answer: "Live sessions reconnect and restore their current state after a dropped connection. Staff can see connection status clearly instead of wondering whether an action went through.",
  },
  {
    question: "Is there a contract?",
    answer: "No long-term contract for the early-access plans. We will confirm the plan and billing terms clearly before your pilot starts.",
  },
] as const;
