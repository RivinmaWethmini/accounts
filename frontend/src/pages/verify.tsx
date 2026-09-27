import { useEffect, useState, type FormEvent } from "react";
import { Link, useSearchParams } from "react-router-dom";
import logo from "../assets/logo.png";
import Button from "../components/button";
import Card from "../components/card";
import Input from "../components/input";
import api from "../lib/api";
import { CheckCircle2, AlertCircle, Loader2 } from "lucide-react";

export default function VerifyEmail() {
  const [searchParams] = useSearchParams();
  const urlToken = searchParams.get("token") || "";

  const [token, setToken] = useState(urlToken);
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<"idle" | "success" | "error">(
    urlToken ? "idle" : "idle"
  );
  const [message, setMessage] = useState("");

  const submitVerification = async (verifyToken: string) => {
    if (!verifyToken.trim()) {
      setStatus("error");
      setMessage("Please enter a verification token.");
      return;
    }

    setLoading(true);
    setStatus("idle");
    setMessage("");

    try {
      const response = await api.post("/api/users/verify", {
        body: JSON.stringify({ token: verifyToken.trim() }),
      });
      const result = await response.json();

      if (response.ok) {
        setStatus("success");
        setMessage("Your email has been successfully verified! You can now log in.");
      } else {
        setStatus("error");
        const errMsg =
          result?.error?.message || "Invalid or expired verification token.";
        setMessage(typeof errMsg === "string" ? errMsg : JSON.stringify(errMsg));
      }
    } catch (err) {
      setStatus("error");
      setMessage("Failed to connect to the server. Please try again.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (urlToken) {
      submitVerification(urlToken);
    }
  }, [urlToken]);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    submitVerification(token);
  };

  return (
    <main className="flex flex-col items-center justify-center min-h-screen bg-gray-100 sm:bg-white px-4 sm:px-0">
      <Card className="shadow-none sm:shadow-sm mx-auto p-5 sm:p-8 w-full max-w-lg text-center">
        <div className="flex flex-col items-center mb-6">
          <img src={logo} width={110} className="my-2" alt="SLIIT Mozilla" />
          <h1 className="text-primary text-2xl font-bold mt-2">Email Verification</h1>
          <p className="text-sm text-gray-500 mt-1">
            Activate your SLIIT Mozilla account
          </p>
        </div>

        {status === "success" ? (
          <div className="py-4">
            <div className="w-16 h-16 bg-green-100 text-green-600 rounded-full flex items-center justify-center mx-auto mb-4">
              <CheckCircle2 size={36} />
            </div>
            <h2 className="text-xl font-semibold text-gray-800">Verification Complete!</h2>
            <p className="text-sm text-gray-600 my-3">{message}</p>
            <Link to="/login">
              <Button className="w-full text-lg mt-4">Proceed to Sign In</Button>
            </Link>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="text-left">
            {status === "error" && (
              <div className="flex items-start gap-2 p-3 mb-4 bg-red-50 border border-red-200 text-red-700 rounded-lg text-sm">
                <AlertCircle size={18} className="shrink-0 mt-0.5" />
                <span>{message}</span>
              </div>
            )}

            <p className="text-sm text-gray-600 mb-4 text-center">
              Please enter the verification token generated during your registration:
            </p>

            <fieldset className="grid mb-4">
              <label htmlFor="token" className="text-sm font-medium mb-1">
                Verification Token
              </label>
              <Input
                type="text"
                id="token"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="Paste your 64-character verification token"
                required
                disabled={loading}
              />
            </fieldset>

            <Button type="submit" disabled={loading} className="w-full text-lg">
              {loading ? (
                <span className="flex items-center justify-center gap-2">
                  <Loader2 className="animate-spin" size={20} /> Verifying...
                </span>
              ) : (
                "Verify Email"
              )}
            </Button>

            <div className="mt-6 text-center text-sm text-gray-500">
              Already verified?{" "}
              <Link to="/login" className="text-black font-semibold hover:underline">
                Sign in
              </Link>
            </div>
          </form>
        )}
      </Card>
    </main>
  );
}
