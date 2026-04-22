import NextAuth from "next-auth";
import GitHub from "next-auth/providers/github";
import Google from "next-auth/providers/google";
import { upsertAppUserFromOAuthIdentity } from "@/lib/app-users";

const authSecret = process.env.NEXTAUTH_SECRET ?? (process.env.NODE_ENV === "development" ? "watcht-dev-secret-change-me" : undefined);

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
      if (!account?.provider || !account.providerAccountId) {
        return true;
      }

      try {
        const appUser = await upsertAppUserFromOAuthIdentity({
          provider: account.provider,
          providerAccountId: account.providerAccountId,
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

      if ((!token.appUserId || !token.appHandle) && account?.provider && account.providerAccountId) {
        const appUser = await upsertAppUserFromOAuthIdentity({
          provider: account.provider,
          providerAccountId: account.providerAccountId,
          displayName: user?.name,
        });

        token.appUserId = appUser.id;
        token.appHandle = appUser.handle;
      }

      return token;
    },
    async session({ session, token }) {
      if (session.user) {
        session.user.appUserId = typeof token.appUserId === "string" ? token.appUserId : undefined;
        session.user.handle = typeof token.appHandle === "string" ? token.appHandle : undefined;
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