"use client";

/**
 * Auth Context — manages JWT token and user state.
 *
 * WHY A CONTEXT?
 *   Without context, every component that needs auth would have to
 *   read localStorage independently. Context centralizes this:
 *   - One place to check if user is logged in
 *   - One place to handle login/logout
 *   - Components just useAuth() to access state
 *
 * HOW IT WORKS:
 *   1. On mount, check localStorage for a token
 *   2. If token exists → user is "logged in"
 *   3. login() saves token to localStorage + state
 *   4. logout() clears token from localStorage + state
 *   5. Components read isLoggedIn and call login/logout
 */

import {
  createContext,
  useContext,
  useState,
  useEffect,
  type ReactNode,
} from "react";
import { getToken, setToken, clearToken, setApiKey } from "./api";

interface AuthState {
  isLoggedIn: boolean;
  loading: boolean;
  login: (token: string, apiKey?: string) => void;
  logout: () => void;
}

const AuthContext = createContext<AuthState>({
  isLoggedIn: false,
  loading: true,
  login: () => {},
  logout: () => {},
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [isLoggedIn, setIsLoggedIn] = useState(false);
  const [loading, setLoading] = useState(true);

  // On mount, check if we already have a token in localStorage
  useEffect(() => {
    const token = getToken();
    setIsLoggedIn(!!token);
    setLoading(false);
  }, []);

  const login = (token: string, apiKey?: string) => {
    setToken(token);
    if (apiKey) setApiKey(apiKey);
    setIsLoggedIn(true);
  };

  const logout = () => {
    clearToken();
    setIsLoggedIn(false);
  };

  return (
    <AuthContext.Provider value={{ isLoggedIn, loading, login, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
