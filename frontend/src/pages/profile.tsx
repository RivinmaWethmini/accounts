import { useEffect, useState } from "react";
import Card from "../components/card";
import { Github, Pencil, Users } from "lucide-react";

import api from "../lib/api";
import logo from "../assets/logo.png";
import type { User } from "../types/user";
import Button from "../components/button";
import RoleChip from "../components/roleChip";
import ProfileImage from "../components/profileImage";

export default function Profile() {
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    (async () => {
      const response = await api.get("/api/users/me");
      const result = await response.json();
      if (!response.ok || !result.data) return;
      setUser(result.data);
    })();
  }, []);

  if (user === null) return;

  const linkGithub = async () => {
    const response = await api.post("/api/connections/github/link");
    const result = await response.json();
    window.location.href = result.data;
  };

  const linkGoogle = async () => {
    const response = await api.get("/api/connections/google/url?redirect=/profile");
    const result = await response.json();
    if (result.data?.url) {
      window.location.href = result.data.url;
    }
  };

  const unlinkConnection = async (provider: string) => {
    if (!confirm(`Are you sure you want to unlink your ${provider} account?`)) return;
    const response = await api.del(`/api/connections/${provider.toLowerCase()}`);
    if (response.ok) {
      window.location.reload();
    }
  };

  const githubConnection = user.connections?.find(
    (connection) => connection.provider.toLowerCase() === "github"
  );

  const googleConnection = user.connections?.find(
    (connection) => connection.provider.toLowerCase() === "google"
  );

  return (
    <div className="grid">
      {/* Banner section */}
      <div className="bg-[#F47624]/15 p-4 px-4 pt-12 sm:p-5 sm:px-10 sm:pt-20 overflow-hidden">
        <img
          src={logo}
          className="float-right opacity-20 w-48 sm:w-72 md:w-96"
        />
      </div>

      {/* Profile header with avatar and edit button */}
      <div className="relative mx-4 sm:mx-6 md:mx-10">
        <ProfileImage
          name={user.name}
          className="absolute top-0 -translate-y-1/2 w-24 h-24 sm:w-24 sm:h-24 md:w-32 md:h-32 lg:w-38 lg:h-38 text-3xl sm:text-3xl md:text-5xl lg:text-6xl"
        />
        <Button
          type="button"
          disabled
          title="Coming soon"
          className="flex items-center gap-2 sm:gap-3 float-right mt-2 sm:mt-3 text-sm sm:text-base px-3 py-1.5 sm:px-4 sm:py-2 opacity-60 cursor-not-allowed"
        >
          <Pencil className="size-3 sm:size-4" />
          <span className="hidden xs:inline sm:inline">Edit profile</span>
          <span className="xs:hidden sm:hidden">Edit</span>
        </Button>
      </div>

      {/* User info section */}
      <div className="mx-4 sm:mx-6 md:mx-10 mt-8 sm:mt-10 md:mt-14">
        <h2 className="text-xl sm:text-2xl md:text-4xl lg:text-5xl font-semibold">
          {user.name}
        </h2>
        <div className="text-xs sm:text-sm md:text-lg lg:text-xl font-extralight text-gray-600">
          {user.email}
        </div>
        {user.roles.length > 0 && (
          <>
            <h5 className="text-base sm:text-lg md:text-xl my-2 sm:my-3 font-medium">
              Roles
            </h5>
            <div className="flex flex-wrap gap-2">
              <RoleChip highlightedRole={true} name={user.roles[0]} />
              {user.roles.slice(1).map((role) => (
                <RoleChip key={role} name={role} />
              ))}
            </div>
          </>
        )}
      </div>
      <Card className="mx-4 sm:mx-6 md:mx-10 my-5">
        <h3 className="text-xl md:text-4xl my-4">Information</h3>
        <div>
          <h5 className="flex gap-2 items-center text-sm sm:text-base md:text-xl my-2 text-primary">
            <Users className="size-4 sm:size-5" />
            <span>Connections</span>
          </h5>
          {(user.connections && user.connections.length > 0) && (
            user.connections.map((connection) => {
              const isGithub =
                connection.provider.toLowerCase() === "github" &&
                !!connection.providerUserName;
              const displayValue =
                connection.providerUserName ||
                connection.providerAccountEmail ||
                connection.providerUserId;

              return (
                <Card
                  key={`${connection.provider}-${connection.providerUserId}`}
                  className="bg-white my-2 sm:my-3 text-xs sm:text-sm md:text-base p-3 sm:p-4 flex items-center justify-between gap-3"
                >
                  <div>
                    <span className="font-semibold capitalize">
                      {connection.provider}
                    </span>
                    {": "}
                    {isGithub ? (
                      <a
                        href={`https://github.com/${connection.providerUserName}`}
                        target="_blank"
                        rel="noreferrer"
                        className="text-blue-600 hover:underline"
                      >
                        {displayValue}
                      </a>
                    ) : (
                      <span>{displayValue}</span>
                    )}
                  </div>
                  <Button
                    type="button"
                    kind="danger-secondary"
                    onClick={() => unlinkConnection(connection.provider)}
                    className="text-xs px-2.5 py-1"
                  >
                    Unlink
                  </Button>
                </Card>
              );
            })
          )}
          {!githubConnection && (
            <Card className="bg-white/60 border border-dashed border-gray-300 my-2 sm:my-3 text-xs sm:text-sm md:text-base p-3 sm:p-4 flex items-center justify-between gap-3">
              <div>
                <div className="font-medium flex items-center gap-2">
                  <Github className="size-4" />
                  <span>Link your GitHub account</span>
                </div>
                <p className="text-gray-500 text-[11px] sm:text-xs mt-1">
                  Connect your GitHub profile to show it here and use it for future sign-ins.
                </p>
              </div>
              <Button
                type="button"
                onClick={linkGithub}
                className="text-xs sm:text-sm px-3 py-1.5 sm:px-4 sm:py-2"
              >
                Link GitHub
              </Button>
            </Card>
          )}
          {!googleConnection && (
            <Card className="bg-white/60 border border-dashed border-gray-300 my-2 sm:my-3 text-xs sm:text-sm md:text-base p-3 sm:p-4 flex items-center justify-between gap-3">
              <div>
                <div className="font-medium flex items-center gap-2">
                  <svg className="w-4 h-4 shrink-0" viewBox="0 0 24 24">
                    <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
                    <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
                    <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z" fill="#FBBC05"/>
                    <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z" fill="#EA4335"/>
                  </svg>
                  <span>Link your Google account</span>
                </div>
                <p className="text-gray-500 text-[11px] sm:text-xs mt-1">
                  Connect your Google account to log in seamlessly with OpenID Connect & PKCE.
                </p>
              </div>
              <Button
                type="button"
                onClick={linkGoogle}
                className="text-xs sm:text-sm px-3 py-1.5 sm:px-4 sm:py-2"
              >
                Link Google
              </Button>
            </Card>
          )}
        </div>
      </Card>
    </div>
  );
}