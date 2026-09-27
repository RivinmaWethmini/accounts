import { useEffect } from "react";
import api from "../lib/api";

export default function Index() {
  useEffect(() => {
    const token = localStorage.getItem("token");
    if (!token) {
      window.location.href = "/login";
      return;
    }

    (async () => {
      try {
        const response = await api.get("/api/session");
        if (response.ok) {
          window.location.href = "/profile";
          return;
        }
      } catch (e) {
        // ignore
      }
      localStorage.removeItem("token");
      window.location.href = "/login";
    })();
  }, []);

  return <></>;
}
