import { useState } from "react";
import { twMerge } from "tailwind-merge";
import api from "../lib/api";
import { useAlert } from "../contexts/alert";

interface GoogleButtonProps {
  text?: string;
  redirect?: string;
  className?: string;
  disabled?: boolean;
}

export default function GoogleButton({
  text = "Continue with Google",
  redirect = "/profile",
  className,
  disabled = false,
}: GoogleButtonProps) {
  const [loading, setLoading] = useState(false);
  const { dispatchAlert } = useAlert();

  const handleGoogleLogin = async () => {
    if (loading || disabled) return;
    setLoading(true);

    try {
      const response = await api.get(
        `/api/auth/google/url?redirect=${encodeURIComponent(redirect)}`
      );
      const result = await response.json();

      if (response.ok && result.data?.url) {
        window.location.href = result.data.url;
      } else {
        const errorMsg =
          result.error?.message ||
          result.message ||
          "Google authentication is temporarily unavailable.";
        dispatchAlert({
          type: "error",
          message: errorMsg,
          position: "top center",
        });
        setLoading(false);
      }
    } catch (err) {
      dispatchAlert({
        type: "error",
        message: "Failed to initiate Google authentication. Please try again.",
        position: "top center",
      });
      setLoading(false);
    }
  };

  return (
    <button
      type="button"
      onClick={handleGoogleLogin}
      disabled={loading || disabled}
      className={twMerge(
        "w-full flex items-center justify-center gap-3 py-2.5 px-4 rounded-sm border border-gray-300 bg-white text-gray-700 text-sm font-medium shadow-xs hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-offset-1 focus:ring-gray-400 transition-colors disabled:opacity-60 disabled:cursor-not-allowed cursor-pointer",
        className
      )}
    >
      <svg
        className="w-4 h-4 shrink-0"
        viewBox="0 0 24 24"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
          fill="#4285F4"
        />
        <path
          d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
          fill="#34A853"
        />
        <path
          d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"
          fill="#FBBC05"
        />
        <path
          d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"
          fill="#EA4335"
        />
      </svg>
      <span>{loading ? "Redirecting to Google..." : text}</span>
    </button>
  );
}
