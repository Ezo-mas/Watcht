import type { Metadata } from "next";
import "./globals.css";
import SiteHeader from "@/components/site-header";
import SiteFooter from "@/components/site-footer";

export const metadata: Metadata = {
  title: "watcht",
  description: "Private-first film and TV tracking platform",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body className="min-h-screen flex flex-col">
        <SiteHeader />
        <main className="mx-auto flex-1 max-w-[96rem] px-5 py-8">{children}</main>
        <SiteFooter />
      </body>
    </html>
  );
}
