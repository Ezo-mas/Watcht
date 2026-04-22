import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./lib/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        canvas: "#0f172a",
        panel: "#111827",
        ink: "#e5e7eb",
        accent: "#f59e0b",
        accentAlt: "#ef4444"
      }
    }
  },
  plugins: [],
};

export default config;
