import NextAuth from "next-auth";
import Credentials from "next-auth/providers/credentials";
import GitHub from "next-auth/providers/github";
import Google from "next-auth/providers/google";
import { getAppUserSummaryById, upsertAppUserFromOAuthIdentity } from "@/lib/app-users";

const isDevelopment = process.env.NODE_ENV === "development";
const localBypassEnabled = isDevelopment && process.env.AUTH_ENABLE_LOCAL_BYPASS !== "false";
const authSecret = process.env.NEXTAUTH_SECRET ?? (isDevelopment ? "watcht-dev-secret-change-me" : undefined);

const providers = [];

if (process.env.GOOGLE_CLIENT_ID && process.env.GOOGLE_CLIENT_SECRET) {
  providers.push(
    Google({
      clientId: process.env.GOOGLE_CLIENT_ID,
      clientSecret: process.env.GOOGLE_CLIENT_SECRET,
    })
  );
}

if (process.env.GITHUB_ID && process.env.GITHUB_SECRET) {
  providers.push(
    GitHub({
      clientId: process.env.GITHUB_ID,
      clientSecret: process.env.GITHUB_SECRET,
    })
  );
}

if (localBypassEnabled) {
  providers.push(
    Credentials({
      name: "Local Dev",
      credentials: {},
      async authorize() {
        // Create or resolve a persistent local dev app user in the DB so the
        // local bypass works against a real record instead of only in-memory.
        const providerAccountId = "local-dev";

        const appUser = await upsertAppUserFromOAuthIdentity({
          provider: "credentials",
          providerAccountId,
          displayName: "Local Dev",
        });

        // Return the provider account id as the user id so resolveProviderAccountId
        // can pick it up; include appUser fields for downstream callbacks.
        return { id: providerAccountId, name: "Local Dev", appUserId: appUser.id, appHandle: appUser.handle };
      },
    })
  );
}

// Basic runtime warnings to help diagnose common misconfigurations
if (providers.length === 0) {
  console.warn("NextAuth: no OAuth providers configured. Set GOOGLE_CLIENT_ID/SECRET or GITHUB_ID/SECRET in .env.local or enable local bypass in development.");
}

if (!process.env.NEXTAUTH_URL) {
  console.warn("NEXTAUTH_URL is not set. OAuth providers require the redirect URL to match the provider app settings (e.g. http://localhost:3000).");
}

if (!authSecret) {
  console.warn("NEXTAUTH_SECRET is not set; using an insecure default in development. Set NEXTAUTH_SECRET in .env.local for production.");
}

function resolveProviderAccountId(account: { providerAccountId?: string } | null | undefined, user: { id?: string } | null | undefined) {
  if (account?.providerAccountId) {
    return account.providerAccountId;
  }

  if (user?.id) {
    return user.id;
  }

  return null;
}

export const { handlers, auth, signIn, signOut } = NextAuth({
  secret: authSecret,
  providers,
  pages: {
    signIn: "/auth/sign-in",
  },
  session: {
    strategy: "jwt",
  },
  callbacks: {
    async signIn({ user, account }) {
      const providerAccountId = resolveProviderAccountId(account, user);
      if (!account?.provider || !providerAccountId) {
        return true;
      }

      const appUser = await upsertAppUserFromOAuthIdentity({
        provider: account.provider,
        providerAccountId,
        displayName: user.name,
      });

      user.appUserId = appUser.id;
      user.appHandle = appUser.handle;
      return true;
    },
    async jwt({ token, user, account }) {
      if (user?.appUserId) {
        token.appUserId = user.appUserId;
      }

      if (user?.appHandle) {
        token.appHandle = user.appHandle;
      }

      const providerAccountId = resolveProviderAccountId(account, user);
      // Avoid DB calls for the local credentials bypass
      if (
        (!token.appUserId || !token.appHandle) &&
        account?.provider &&
        providerAccountId &&
        !(account.provider === "credentials" && localBypassEnabled)
      ) {
        const appUser = await upsertAppUserFromOAuthIdentity({
          provider: account.provider,
          providerAccountId,
          displayName: user?.name,
        });

        token.appUserId = appUser.id;
        token.appHandle = appUser.handle;
      }

      return token;
    },
    async session({ session, token }) {
      if (session.user && typeof token.appUserId === "string") {
        session.user.appUserId = token.appUserId;

        // For local dev bypass, avoid DB lookups and use token fallback values
        if (localBypassEnabled && token.appUserId.startsWith("local-")) {
          session.user.handle = typeof token.appHandle === "string" ? token.appHandle : undefined;
          session.user.displayName = typeof token.appHandle === "string" ? token.appHandle : undefined;
          session.user.isPrivate = true;
        } else {
          const appUser = await getAppUserSummaryById(token.appUserId);
          if (appUser) {
            session.user.handle = appUser.handle;
            session.user.displayName = appUser.displayName ?? undefined;
            session.user.isPrivate = appUser.isPrivate;
            session.user.joinedAt = appUser.joinedAt?.toISOString() ?? undefined;
          } else {
            session.user.handle = typeof token.appHandle === "string" ? token.appHandle : undefined;
          }
        }
      }

      return session;
    },
    authorized({ auth: session, request: { nextUrl } }) {
      if (nextUrl.pathname.startsWith("/dashboard") || nextUrl.pathname.startsWith("/profile")) {
        return !!session?.user;
      }

      return true;
    },
  },
});