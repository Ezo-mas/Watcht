import Image from "next/image";

type ProfileHeroData = {
  imageUrl: string | null;
  name: string;
  handle: string | null;
  badge: string | null;
  followersLabel: string;
  followingLabel: string;
  locationLabel: string | null;
  privacyLabel: string;
};

type ProfileCardItem = {
  label: string;
  title: string;
  body: string;
};

type ProfileListItem = {
  title: string;
  subtitle: string;
  meta?: string;
};

type ProfileCommentItem = {
  author: string;
  body: string;
  date: string;
};

type ProfileCardsBlock = {
  id: string;
  kind: "cards";
  title: string;
  subtitle?: string;
  cards: ProfileCardItem[];
};

type ProfileListBlock = {
  id: string;
  kind: "list";
  title: string;
  subtitle?: string;
  items: ProfileListItem[];
  emptyText: string;
  timeLabel?: string;
};

type ProfileCommentsBlock = {
  id: string;
  kind: "comments";
  title: string;
  subtitle?: string;
  timeframe: string;
  items: ProfileCommentItem[];
  emptyText: string;
};

export type ProfilePageBlock = ProfileCardsBlock | ProfileListBlock | ProfileCommentsBlock;

type ProfilePageProps = {
  hero: ProfileHeroData;
  blocks: ProfilePageBlock[];
};

function CardShell({ children, accent = "from-rose-500/70 to-amber-400/70" }: { children: React.ReactNode; accent?: string }) {
  return (
    <article className="overflow-hidden rounded-3xl border border-white/10 bg-slate-950/80 shadow-[0_18px_60px_rgba(0,0,0,0.35)]">
      <div className={`h-1 w-full bg-gradient-to-r ${accent}`} />
      <div className="p-4">{children}</div>
    </article>
  );
}

function HeroAvatar({ imageUrl, name }: { imageUrl: string | null; name: string }) {
  const initial = name.trim().charAt(0).toUpperCase() || "A";

  if (imageUrl) {
    return <Image src={imageUrl} alt={name} width={96} height={96} unoptimized className="h-24 w-24 rounded-full border-4 border-white/90 object-cover shadow-lg" />;
  }

  return (
    <div className="flex h-24 w-24 items-center justify-center rounded-full border-4 border-white/90 bg-gradient-to-br from-fuchsia-500 to-rose-600 text-4xl font-bold text-white shadow-lg">
      {initial}
    </div>
  );
}

export default function ProfilePage({ hero, blocks }: ProfilePageProps) {
  return (
    <section className="space-y-6 pb-12">
      <header className="overflow-hidden rounded-[2rem] border border-white/10 bg-[radial-gradient(circle_at_18%_18%,rgba(255,255,255,0.12),transparent_20%),radial-gradient(circle_at_82%_16%,rgba(255,64,64,0.25),transparent_22%),linear-gradient(135deg,#260309_0%,#110405_45%,#0b0b0c_100%)] shadow-[0_18px_70px_rgba(0,0,0,0.45)]">
        <div className="flex flex-col gap-6 p-6 sm:p-8 lg:flex-row lg:items-end lg:justify-between">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-end">
            <HeroAvatar imageUrl={hero.imageUrl} name={hero.name} />
            <div>
              <div className="flex flex-wrap items-center gap-3">
                <h1 className="text-4xl font-black tracking-tight text-white sm:text-5xl">{hero.name}</h1>
                {hero.badge ? (
                  <span className="rounded-full border border-rose-300/30 bg-rose-500/90 px-3 py-1 text-sm font-semibold uppercase tracking-[0.18em] text-white">
                    {hero.badge}
                  </span>
                ) : null}
              </div>
              <p className="mt-2 text-sm text-white/85">{hero.handle ? `@${hero.handle}` : "No handle yet"}</p>
              <div className="mt-3 flex flex-wrap items-center gap-2 text-sm text-white/90">
                <span className="rounded-full border border-white/15 bg-black/20 px-3 py-1">{hero.followersLabel}</span>
                <span className="rounded-full border border-white/15 bg-black/20 px-3 py-1">{hero.followingLabel}</span>
                <span className="rounded-full border border-white/15 bg-black/20 px-3 py-1">{hero.privacyLabel}</span>
                {hero.locationLabel ? <span className="rounded-full border border-white/15 bg-black/20 px-3 py-1">{hero.locationLabel}</span> : null}
              </div>
            </div>
          </div>

          <div className="max-w-sm rounded-2xl border border-white/10 bg-black/30 px-4 py-3 text-sm text-white/85 backdrop-blur">
            <p className="text-xs uppercase tracking-[0.22em] text-white/55">Profile</p>
            <p className="mt-1">Built as a block-driven page so you can reorder or swap sections later without rewriting the route.</p>
          </div>
        </div>
      </header>

      {blocks.map((block) => {
        if (block.kind === "cards") {
          return (
            <section key={block.id} className="space-y-3">
              <div className="flex items-end justify-between gap-3">
                <div>
                  <h2 className="text-2xl font-bold text-white">{block.title}</h2>
                  {block.subtitle ? <p className="text-sm text-slate-400">{block.subtitle}</p> : null}
                </div>
              </div>
              <div className={`grid gap-3 ${block.cards.length >= 4 ? "lg:grid-cols-4" : "md:grid-cols-2 xl:grid-cols-3"}`}>
                {block.cards.map((card) => (
                  <CardShell key={`${block.id}-${card.label}`} accent="from-fuchsia-500/80 via-rose-400/70 to-amber-300/70">
                    <p className="text-[11px] uppercase tracking-[0.22em] text-slate-400">{card.label}</p>
                    <p className="mt-2 text-lg font-semibold text-white">{card.title}</p>
                    <p className="mt-2 text-sm text-slate-300">{card.body}</p>
                  </CardShell>
                ))}
              </div>
            </section>
          );
        }

        if (block.kind === "list") {
          return (
            <section key={block.id} className="space-y-3">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h2 className="text-2xl font-bold text-white">{block.title}</h2>
                  {block.subtitle ? <p className="text-sm text-slate-400">{block.subtitle}</p> : null}
                </div>
                {block.timeLabel ? (
                  <span className="rounded-full border border-white/10 bg-slate-900/60 px-3 py-1 text-xs uppercase tracking-[0.18em] text-slate-200">
                    {block.timeLabel}
                  </span>
                ) : null}
              </div>

              {block.items.length === 0 ? (
                <CardShell accent="from-slate-500/70 to-slate-300/70">
                  <p className="text-sm text-slate-300">{block.emptyText}</p>
                </CardShell>
              ) : (
                <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                  {block.items.map((item) => (
                    <CardShell key={`${block.id}-${item.title}`} accent="from-slate-500/50 to-rose-500/50">
                      <p className="text-sm font-semibold text-white">{item.title}</p>
                      <p className="mt-1 text-sm text-slate-300">{item.subtitle}</p>
                      {item.meta ? <p className="mt-2 text-[11px] uppercase tracking-[0.2em] text-slate-400">{item.meta}</p> : null}
                    </CardShell>
                  ))}
                </div>
              )}
            </section>
          );
        }

        return (
          <section key={block.id} className="space-y-3">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h2 className="text-2xl font-bold text-white">{block.title}</h2>
                {block.subtitle ? <p className="text-sm text-slate-400">{block.subtitle}</p> : null}
              </div>
              <span className="rounded-full border border-white/10 bg-slate-900/60 px-3 py-1 text-xs uppercase tracking-[0.18em] text-slate-200">
                {block.timeframe}
              </span>
            </div>

            {block.items.length === 0 ? (
              <CardShell accent="from-slate-500/70 to-slate-300/70">
                <p className="text-sm text-slate-300">{block.emptyText}</p>
              </CardShell>
            ) : (
              <div className="space-y-3">
                {block.items.map((comment) => (
                  <CardShell key={`${block.id}-${comment.author}-${comment.date}`} accent="from-amber-500/60 to-rose-500/60">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-sm font-semibold text-white">{comment.author}</p>
                        <p className="mt-1 text-sm text-slate-300">{comment.body}</p>
                      </div>
                      <p className="whitespace-nowrap text-[11px] uppercase tracking-[0.18em] text-slate-400">{comment.date}</p>
                    </div>
                  </CardShell>
                ))}
              </div>
            )}
          </section>
        );
      })}
    </section>
  );
}
