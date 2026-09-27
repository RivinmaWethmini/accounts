import { useEffect, useState, type FormEvent } from "react";
import { Link, useSearchParams } from "react-router-dom";
import logo from "../assets/logo.png";
import Button from "../components/button";
import Card from "../components/card";
import api from "../lib/api";
import { CheckCircle2, AlertCircle, Loader2, Mail, ArrowRight, ShieldCheck, RefreshCw } from "lucide-react";

export default function VerifyEmail() {
  const [searchParams] = useSearchParams();
  const initialEmail = searchParams.get("email") || "";
  const initialCode = searchParams.get("code") || searchParams.get("token") || "";

  const [email, setEmail] = useState(initialEmail);
  const [otp, setOtp] = useState(initialCode);
  const [loading, setLoading] = useState(false);
  const [resending, setResending] = useState(false);
  const [status, setStatus] = useState<"idle" | "success" | "error">("idle");
  const [message, setMessage] = useState("");
  const [resendStatus, setResendStatus] = useState<"idle" | "success" | "error">("idle");
  const [resendMessage, setResendMessage] = useState("");
  const [resendCooldown, setResendCooldown] = useState(0);

  // Handle countdown for resend button
  useEffect(() => {
    if (resendCooldown <= 0) return;
    const timer = setInterval(() => {
      setResendCooldown((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [resendCooldown]);

  const submitVerification = async (codeToVerify: string, targetEmail: string) => {
    const cleanedCode = codeToVerify.trim();
    if (!cleanedCode) {
      setStatus("error");
      setMessage("Please enter the 6-digit verification code.");
      return;
    }

    if (cleanedCode.length < 6) {
      setStatus("error");
      setMessage("Verification code must be 6 digits.");
      return;
    }

    setLoading(true);
    setStatus("idle");
    setMessage("");

    try {
      const response = await api.post("/api/users/verify", {
        body: JSON.stringify({
          code: cleanedCode,
          email: targetEmail.trim(),
        }),
      });
      const result = await response.json();

      if (response.ok) {
        setStatus("success");
        setMessage("Your email has been successfully verified! You can now log in.");
      } else {
        setStatus("error");
        const errMsg =
          result?.error?.message || "Invalid or expired verification code.";
        setMessage(typeof errMsg === "string" ? errMsg : JSON.stringify(errMsg));
      }
    } catch {
      setStatus("error");
      setMessage("Unable to connect to the server. Please check your internet connection.");
    } finally {
      setLoading(false);
    }
  };

  // If a 6-digit code is passed via URL query, auto-verify
  useEffect(() => {
    if (initialCode && initialCode.length === 6) {
      submitVerification(initialCode, initialEmail);
    }
  }, [initialCode]);

  const handleOtpChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    // Only accept numeric digits, maximum 6 characters
    const numericOnly = e.target.value.replace(/\D/g, "").slice(0, 6);
    setOtp(numericOnly);

    // Auto-clear error on input
    if (status === "error") {
      setStatus("idle");
      setMessage("");
    }

    // Auto submit once 6 digits are typed if email is known
    if (numericOnly.length === 6) {
      submitVerification(numericOnly, email);
    }
  };

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    submitVerification(otp, email);
  };

  const handleResend = async () => {
    const targetEmail = email.trim();
    if (!targetEmail) {
      setResendStatus("error");
      setResendMessage("Please enter your email address to resend code.");
      return;
    }

    setResending(true);
    setResendStatus("idle");
    setResendMessage("");

    try {
      const response = await api.post("/api/users/resend-verification", {
        body: JSON.stringify({ email: targetEmail }),
      });
      const result = await response.json();

      if (response.ok) {
        setResendStatus("success");
        setResendMessage("A new 6-digit code has been sent to your email!");
        setResendCooldown(60); // 60 seconds cooldown
      } else {
        setResendStatus("error");
        const errMsg = result?.error?.message || "Failed to resend code.";
        setResendMessage(typeof errMsg === "string" ? errMsg : JSON.stringify(errMsg));
      }
    } catch {
      setResendStatus("error");
      setResendMessage("Failed to reach server. Please try again.");
    } finally {
      setResending(false);
    }
  };

  return (
    <main className="flex flex-col items-center justify-center min-h-screen bg-gray-100 sm:bg-white px-4 sm:px-0">
      <Card className="shadow-none sm:shadow-sm mx-auto p-6 sm:p-8 w-full max-w-md text-center">
        <div className="flex flex-col items-center mb-6">
          <img src={logo} width={110} className="my-2" alt="SLIIT Mozilla" />
          <h1 className="text-primary text-2xl font-bold mt-2">Enter Verification Code</h1>
          <p className="text-sm text-gray-500 mt-1">
            We sent a 6-digit OTP code to your email
          </p>
        </div>

        {status === "success" ? (
          <div className="py-4">
            <div className="w-16 h-16 bg-green-100 text-green-600 rounded-full flex items-center justify-center mx-auto mb-4">
              <CheckCircle2 size={36} />
            </div>
            <h2 className="text-xl font-semibold text-gray-800">Account Activated!</h2>
            <p className="text-sm text-gray-600 my-3">{message}</p>
            <Link to="/login">
              <Button className="w-full text-lg mt-4 flex items-center justify-center gap-2">
                <span>Proceed to Sign In</span>
                <ArrowRight size={18} />
              </Button>
            </Link>
          </div>
        ) : (
          <div className="text-left">
            {email && (
              <div className="flex items-center gap-3 p-3.5 mb-5 bg-orange-50 border border-orange-200 text-orange-950 rounded-xl text-xs">
                <Mail className="size-5 shrink-0 text-primary" />
                <div className="overflow-hidden">
                  <span className="font-semibold text-orange-900 uppercase tracking-wide block">Code sent to</span>
                  <span className="text-gray-700 font-medium truncate block">{email}</span>
                </div>
              </div>
            )}

            {status === "error" && (
              <div className="flex items-start gap-2 p-3 mb-4 bg-red-50 border border-red-200 text-red-700 rounded-lg text-sm">
                <AlertCircle size={18} className="shrink-0 mt-0.5" />
                <span>{message}</span>
              </div>
            )}

            <form onSubmit={handleSubmit} className="mb-5">
              <div className="mb-4">
                {!email && (
                  <div className="mb-3">
                    <label htmlFor="email" className="block text-xs font-semibold text-gray-600 uppercase mb-1">
                      Email Address
                    </label>
                    <input
                      type="email"
                      id="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                      placeholder="your.email@example.com"
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary/50"
                      required
                    />
                  </div>
                )}

                <label htmlFor="otp" className="block text-xs font-semibold text-gray-600 uppercase mb-2 text-center">
                  6-Digit OTP Code
                </label>
                <div className="flex justify-center">
                  <input
                    type="text"
                    id="otp"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    pattern="[0-9]*"
                    maxLength={6}
                    value={otp}
                    onChange={handleOtpChange}
                    placeholder="000000"
                    disabled={loading}
                    autoFocus
                    className="w-full max-w-[260px] text-center text-3xl font-mono font-bold tracking-[0.45em] py-3 px-4 border-2 border-primary/40 focus:border-primary rounded-xl focus:outline-none focus:ring-4 focus:ring-primary/20 transition-all bg-orange-50/30 text-gray-900 placeholder:text-gray-300"
                  />
                </div>
                <p className="text-[11px] text-gray-400 text-center mt-2">
                  Code expires in 10 minutes. Check your spam/junk folder if needed.
                </p>
              </div>

              <Button
                type="submit"
                disabled={loading || otp.length < 6}
                className="w-full text-base py-2.5 mt-2 flex items-center justify-center gap-2"
              >
                {loading ? (
                  <>
                    <Loader2 className="animate-spin" size={18} />
                    <span>Verifying Code...</span>
                  </>
                ) : (
                  <>
                    <ShieldCheck size={18} />
                    <span>Verify & Activate</span>
                  </>
                )}
              </Button>
            </form>

            <div className="border-t border-gray-200 pt-4 mt-4 text-center">
              {resendStatus === "success" && (
                <div className="p-2 mb-3 bg-green-50 border border-green-200 text-green-700 rounded-lg text-xs">
                  {resendMessage}
                </div>
              )}
              {resendStatus === "error" && (
                <div className="p-2 mb-3 bg-red-50 border border-red-200 text-red-700 rounded-lg text-xs">
                  {resendMessage}
                </div>
              )}

              <div className="flex items-center justify-center gap-1 text-xs text-gray-600">
                <span>Didn't receive the code?</span>
                <button
                  type="button"
                  onClick={handleResend}
                  disabled={resending || resendCooldown > 0}
                  className="text-primary font-semibold hover:underline flex items-center gap-1 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  {resending ? (
                    <>
                      <Loader2 className="animate-spin size-3" />
                      <span>Sending...</span>
                    </>
                  ) : resendCooldown > 0 ? (
                    <span>Resend in {resendCooldown}s</span>
                  ) : (
                    <>
                      <RefreshCw className="size-3" />
                      <span>Resend Code</span>
                    </>
                  )}
                </button>
              </div>
            </div>

            <div className="mt-5 text-center text-xs text-gray-500">
              Already verified?{" "}
              <Link to="/login" className="text-black font-semibold hover:underline">
                Sign in
              </Link>
            </div>
          </div>
        )}
      </Card>
    </main>
  );
}
