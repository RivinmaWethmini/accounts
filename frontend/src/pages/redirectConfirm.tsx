import { useSearchParams } from "react-router-dom";
import logo from "../assets/logo.png";
import Button from "../components/button";
import Card from "../components/card";

export default function RedirectConfirm() {
  const [searchParams] = useSearchParams();
  const redirect = searchParams.get("redirect") ?? "";

  let host = "";
  try {
    host = new URL(redirect).host;
  } catch {
    host = redirect;
  }

  const handleContinue = () => {
    // Full browser navigation (not fetch) so the refreshToken cookie is sent
    // and the backend's redirect response is followed by the browser itself.
    window.location.href = `/api/authorize/confirm?redirect=${encodeURIComponent(
      redirect
    )}`;
  };

  return (
    <main className="flex flex-col items-center justify-center min-h-screen bg-gray-100 sm:bg-white px-4 sm:px-0">
      <Card className="shadow-none sm:shadow-sm mx-auto p-5 sm:p-8 w-full max-w-lg">
        <div className="grid w-full min-w-0">
          <div>
            <img src={logo} width={110} className="my-3" />
            <h1 className="text-primary text-2xl">Continue to this site?</h1>
            <h5 className="text-sm text-gray-500 mt-1">
              You're about to be redirected to a site outside SLIIT Mozilla
              Accounts.
            </h5>
          </div>

          <div className="my-5 border border-black rounded-2xl p-4">
            <p className="text-xs text-gray-500">Destination</p>
            <p className="text-lg font-medium break-all">{host}</p>
          </div>

          <p className="text-sm text-gray-500 mb-4">
            Only continue if you recognize and trust this destination. You
            won't be asked again for this site.
          </p>

          <Button type="button" className="text-xl my-2" onClick={handleContinue}>
            Continue
          </Button>

          <a
            href="/"
            className="text-center text-sm text-gray-500 mt-2 underline"
          >
            Cancel
          </a>
        </div>
      </Card>
    </main>
  );
}