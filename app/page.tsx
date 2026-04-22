import Link from "next/link";

export default function HomePage() {
  return (
    <section className="grid gap-8 md:grid-cols-[1.4fr_1fr]">
      <div className="card rounded-2xl p-8">
        <p className="text-sm uppercase tracking-[0.2em] text-amber-300">watcht alpha</p>
        <h1 className="mt-3 text-4xl font-bold leading-tight text-white">Track films and shows with depth, not noise.</h1>
        <p className="mt-4 max-w-2xl text-slate-300">
          Lean scaffold inspired by classic tracking tools. Next steps will connect auth, metadata ingestion,
          and progress workflows.
        </p>
        <div className="mt-6 flex gap-3">
          <Link href="/dashboard" className="rounded-md bg-amber-500 px-4 py-2 font-medium text-slate-950 transition hover:bg-amber-400">
            Open dashboard
          </Link>
          <Link href="/auth/sign-in" className="rounded-md border border-white/20 px-4 py-2 text-slate-100 transition hover:border-white/40">
            Sign in shell
          </Link>
        </div>
      </div>
      <aside className="card rounded-2xl p-6">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-slate-300">MVP in progress</h2>
        <ul className="mt-4 space-y-2 text-sm text-slate-300">
          <li>Watchlist and watch history</li>
          <li>Episode progress tracking</li>
          <li>Ratings and short reviews</li>
          <li>Stats dashboard</li>
        </ul>
      </aside>
    </section>
  );
}
