import { useEffect, useRef, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import api from "../lib/api";
import { useAlert } from "../contexts/alert";
import Card from "../components/card";

export default function OAuthCallback() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { dispatchAlert } = useAlert();
  const [status, setStatus] = useState<"processing" | "success" | "error">("processing");
  const [errorMessage, setErrorMessage] = useState("");
  const executedRef = useRef(false);

  useEffect(() => {
    if (executedRef.current) return;
    executedRef.current = true;

    const code = searchParams.get("code");
    const state = searchParams.get("state");
    const error = searchParams.get("error");

    if (error) {
      setStatus("error");
      const msg = `Google authentication failed: ${error}`;
      setErrorMessage(msg);
      dispatchAlert({ type: "error", message: msg, position: "top center" });
      setTimeout(() => navigate("/login"), 3000);
      return;
    }

    if (!code || !state) {
      setStatus("error");
      const msg = "Invalid OAuth callback: missing authorization code or state token.";
      setErrorMessage(msg);
      dispatchAlert({ type: "error", message: msg, position: "top center" });
      setTimeout(() => navigate("/login"), 3000);
      return;
    }

    const exchangeCode = async () => {
      try {
        const response = await api.post("/api/auth/google/callback", {
          body: JSON.stringify({ code, state }),
        });

        const result = await response.json();

        if (response.ok && result.data?.token) {
          setStatus("success");
          localStorage.setItem("token", result.data.token);
          dispatchAlert({
            type: "success",
            message: "Authentication successful! Redirecting...",
            position: "top center",
          });

          // Refresh and redirect to profile or home
          setTimeout(() => {
            window.location.href = "/profile";
          }, 800);
        } else {
          setStatus("error");
          const msg =
            result.error?.message ||
            result.message ||
            "Failed to complete Google authentication.";
          setErrorMessage(msg);
          dispatchAlert({ type: "error", message: msg, position: "top center" });
          setTimeout(() => navigate("/login"), 3500);
        }
      } catch (err) {
        setStatus("error");
        const msg = "Network error while completing authentication. Please try again.";
        setErrorMessage(msg);
        dispatchAlert({ type: "error", message: msg, position: "top center" });
        setTimeout(() => navigate("/login"), 3500);
      }
    };

    exchangeCode();
  }, [searchParams, navigate, dispatchAlert]);

  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 p-4">
      <Card className="w-full max-w-md p-8 text-center bg-white shadow-md">
        {status === "processing" && (
          <div className="flex flex-col items-center gap-4">
            <div className="w-10 h-10 border-4 border-primary border-t-transparent rounded-full animate-spin" />
            <h2 className="text-xl font-semibold text-gray-800">
              Verifying Google Authentication...
            </h2>
            <p className="text-sm text-gray-500">
              Completing secure PKCE exchange and validating security claims.
            </p>
          </div>
        )}

        {status === "success" && (
          <div className="flex flex-col items-center gap-3">
            <div className="w-12 h-12 bg-green-100 text-green-600 rounded-full flex items-center justify-center text-2xl font-bold">
              ✓
            </div>
            <h2 className="text-xl font-semibold text-gray-800">
              Login Successful!
            </h2>
            <p className="text-sm text-gray-500">
              Redirecting to your account profile...
            </p>
          </div>
        )}

        {status === "error" && (
          <div className="flex flex-col items-center gap-3">
            <div className="w-12 h-12 bg-red-100 text-red-600 rounded-full flex items-center justify-center text-2xl font-bold">
              ✕
            </div>
            <h2 className="text-xl font-semibold text-red-600">
              Authentication Error
            </h2>
            <p className="text-sm text-gray-600">{errorMessage}</p>
            <button
              onClick={() => navigate("/login")}
              className="mt-4 px-4 py-2 bg-black text-white text-sm rounded-sm hover:opacity-80 transition-opacity"
            >
              Return to Login
            </button>
          </div>
        )}
      </Card>
    </div>
  );
}
