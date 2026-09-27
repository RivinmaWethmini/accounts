import { useRef, useState, type FormEvent } from "react";
import { Link, useSearchParams } from "react-router-dom";
import logo from "../assets/logo.png";
import Button from "../components/button";
import Card from "../components/card";
import api from "../lib/api";
import Input from "../components/input";
import { useAlert } from "../contexts/alert";
import GoogleButton from "../components/googleButton";

export default function Login() {
  const [searchParams] = useSearchParams();
  const { dispatchAlert } = useAlert();

  const emailRef = useRef<HTMLInputElement>(null);
  const [emailError, setEmailError] = useState("");
  const passwordRef = useRef<HTMLInputElement>(null);
  const [passwordError, setPasswordError] = useState("");
  const [unverifiedWarning, setUnverifiedWarning] = useState(false);

  const validateEmail = () => {
    const input = emailRef.current;
    if (!input?.checkValidity()) {
      if (input?.validity.typeMismatch) setEmailError("Invalid email");
      else setEmailError(input?.validationMessage || "");
    } else {
      setEmailError("");
    }
  };

  const validatePassword = () => {
    const input = passwordRef.current;
    if (!input?.checkValidity()) {
      setPasswordError(input?.validationMessage || "");
    } else {
      setPasswordError("");
    }
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    validateEmail();
    validatePassword();

    const emailInput = emailRef.current;
    const passwordInput = passwordRef.current;

    if (!emailInput?.checkValidity() || !passwordInput?.checkValidity()) return;

    try {
      const response = await api.post("/api/login", {
        body: JSON.stringify({
          email: emailInput?.value.trim(),
          password: passwordInput?.value.trim(),
        }),
      });
      const result = await response.json();
      if (response.ok && result.data) {
        localStorage.setItem("token", result.data.token);
        const redirect = searchParams.get("redirect");
        if (redirect) window.location.href = redirect;
        else window.location.href = "/";
      } else {
        const errorMessage =
          result?.error?.message || "Something unexpected happened";
        const displayMessage = Array.isArray(errorMessage)
          ? errorMessage[0]?.reason
          : errorMessage;

        if (
          response.status === 403 &&
          typeof displayMessage === "string" &&
          displayMessage.includes("Email not verified")
        ) {
          setUnverifiedWarning(true);
        } else {
          setUnverifiedWarning(false);
        }

        dispatchAlert({
          type: "error",
          message: displayMessage,
          position: "top center",
        });
      }
    } catch (error) {
      dispatchAlert({
        type: "error",
        message: "Something unexpected happened",
        position: "top center",
      });
    }
  };

  return (
    <main className="flex flex-col items-center justify-center min-h-screen bg-gray-100 sm:bg-white px-4 sm:px-0">
      <Card className="shadow-none sm:shadow-sm mx-auto p-5 sm:p-8 w-full max-w-lg">
        <form className="grid w-full min-w-0" onSubmit={handleSubmit} noValidate>
          <div>
            <img src={logo} width={110} className="my-3" />
            <h1 className="text-primary text-2xl">Welcome Back!</h1>
            <h5 className="text-sm text-gray-500 mt-1">Sign in to continue</h5>
          </div>
          <div className="flex my-5 border border-black rounded-2xl p-1">
            <Link
              className="w-full bg-black text-white rounded-xl p-2 cursor-pointer text-center"
              to={`/login?${searchParams.toString()}`}
            >
              Sign in
            </Link>
            <Link
              className="w-full text-primary p-2 cursor-pointer text-center"
              to={`/signup?${searchParams.toString()}`}
            >
              Sign up
            </Link>
          </div>

          {unverifiedWarning && (
            <div className="flex items-center justify-between gap-3 p-3 my-2 bg-amber-50 border border-amber-200 text-amber-900 rounded-lg text-sm">
              <div>
                <p className="font-semibold">Account Not Verified</p>
                <p className="text-xs text-amber-700 mt-0.5">Please verify your email before logging in.</p>
              </div>
              <Link
                to="/verify"
                className="shrink-0 bg-black text-white px-3 py-1.5 rounded-lg text-xs font-medium hover:bg-gray-800"
              >
                Verify Now
              </Link>
            </div>
          )}

          <div className="my-4 flex flex-col gap-4">
            <fieldset className="grid">
              <label htmlFor="email">Email</label>
              <Input
                type="email"
                id="email"
                error={emailError}
                ref={emailRef}
                required
                onBlur={validateEmail}
              />
            </fieldset>
            <fieldset className="grid">
              <label htmlFor="password">Password</label>
              <Input
                type="password"
                id="password"
                error={passwordError}
                ref={passwordRef}
                required
                onBlur={validatePassword}
                showToggle
              />
            </fieldset>
          </div>
          <Button type="submit" className="text-xl my-2">
            Sign in
          </Button>

          <div className="relative flex py-3 items-center">
            <div className="grow border-t border-gray-300"></div>
            <span className="shrink mx-4 text-gray-400 text-xs uppercase tracking-wider">or continue with</span>
            <div className="grow border-t border-gray-300"></div>
          </div>

          <GoogleButton redirect={searchParams.get("redirect") || "/profile"} text="Sign in with Google" />
        </form>
      </Card>
    </main>
  );
}
