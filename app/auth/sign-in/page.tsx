import { signIn } from "@/auth";

const providers = [
  {
    id: "google",
    label: "Google",
    configured: Boolean(process.env.GOOGLE_CLIENT_ID && process.env.GOOGLE_CLIENT_SECRET),
  },
  {
    id: "github",
    label: "GitHub",
    configured: Boolean(process.env.GITHUB_ID && process.env.GITHUB_SECRET),
  },
] as const;

export default function SignInPage() {
  return (
    <section className="mx-auto max-w-xl rounded-2xl border border-white/10 bg-slate-900/70 p-8">
      <p className="text-xs uppercase tracking-[0.18em] text-amber-300">auth shell</p>
      <h1 className="mt-2 text-3xl font-bold text-white">Sign in to watcht</h1>
      <p className="mt-3 text-sm text-slate-300">
        OAuth is enabled with Google and GitHub. Configure provider secrets in .env.local to activate sign-in.
      </p>

      <div className="mt-6 space-y-3">
        {providers.map((provider) => (
          <form
            key={provider.id}
            action={async () => {
              "use server";

              if (!provider.configured) {
                return;
              }

              await signIn(provider.id, { redirectTo: "/dashboard" });
            }}
          >
            <button
              type="submit"
              disabled={!provider.configured}
              className="w-full rounded-md border border-white/20 px-4 py-2 text-left text-slate-100 transition hover:border-white/40 disabled:cursor-not-allowed disabled:opacity-60"
            >
              Continue with {provider.label}
              {!provider.configured ? " (configure env first)" : ""}
            </button>
          </form>
        ))}
      </div>
    </section>
  );
}
