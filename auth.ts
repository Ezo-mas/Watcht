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
        return { id: "local-dev", name: "Local Dev" };
      },
    })
  );
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

      try {
        const appUser = await upsertAppUserFromOAuthIdentity({
          provider: account.provider,
          providerAccountId,
          displayName: user.name,
        });

        user.appUserId = appUser.id;
        user.appHandle = appUser.handle;
        return true;
      } catch (error) {
        console.error("Unable to sync OAuth user with app_users", error);
        return false;
      }
    },
    async jwt({ token, user, account }) {
      if (user?.appUserId) {
        token.appUserId = user.appUserId;
      }

      if (user?.appHandle) {
        token.appHandle = user.appHandle;
      }

      const providerAccountId = resolveProviderAccountId(account, user);
      if ((!token.appUserId || !token.appHandle) && account?.provider && providerAccountId) {
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

      return session;
    },
    authorized({ auth: session, request: { nextUrl } }) {
      if (nextUrl.pathname.startsWith("/dashboard")) {
        return !!session?.user;
      }

      return true;
    },
  },
});