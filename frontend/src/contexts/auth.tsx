import {
  createContext,
  useContext,
  useState,
  useEffect,
  type ReactNode,
} from "react";
import type { User } from "../types/user";
import jwt from "../utils/jwt";

type AuthContextProps = {
  token: string | null;
  user: Partial<User> | null;
};

const AuthContext = createContext<AuthContextProps>({
  token: null,
  user: null,
});

export const AuthProvider = ({ children }: { children: ReactNode }) => {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem("token"));
  const [user, setUser] = useState<Partial<User> | null>(() => {
    const initialToken = localStorage.getItem("token");
    if (initialToken) {
      const decoded = jwt.decode(initialToken);
      if (decoded?.payload) return decoded.payload as unknown as User;
    }
    return null;
  });

  useEffect(() => {
    const syncAuth = () => {
      const currentToken = localStorage.getItem("token");
      setToken(currentToken);
      if (currentToken) {
        const decoded = jwt.decode(currentToken);
        if (decoded?.payload) {
          setUser(decoded.payload as unknown as User);
        } else {
          setUser(null);
        }
      } else {
        setUser(null);
      }
    };

    window.addEventListener("storage", syncAuth);
    syncAuth();
    return () => window.removeEventListener("storage", syncAuth);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        token,
        user,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  return useContext(AuthContext);
};
