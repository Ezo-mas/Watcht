import Link from "next/link";

const footerLinks = [
  { href: "#", label: "About" },
  { href: "#", label: "Activate Device" },
  { href: "#", label: "Branding" },
  { href: "#", label: "API" },
  { href: "#", label: "Create an App" },
  { href: "#", label: "Import Movie" },
  { href: "#", label: "Import TV Show" },
  { href: "#", label: "Help" },
  { href: "#", label: "Terms" },
  { href: "#", label: "Privacy" },
];

export default function SiteFooter() {
  return (
    <footer className="border-t border-white/10 bg-black/60">
      <div className="mx-auto flex max-w-[96rem] flex-wrap items-center gap-x-8 gap-y-4 px-5 py-6 text-sm uppercase tracking-wide text-slate-300">
        {footerLinks.map((item) => (
          <Link key={item.label} href={item.href} className="transition hover:text-white">
            {item.label}
          </Link>
        ))}
      </div>
    </footer>
  );
}
