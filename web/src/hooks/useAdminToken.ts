import { useCallback, useState } from "react";

const STORAGE_KEY = "sharpen-live-admin-token";

export function useAdminToken(): [string, (token: string) => void, () => void] {
  const [token, setTokenState] = useState(() => {
    if (typeof window === "undefined") {
      return "";
    }
    return window.localStorage.getItem(STORAGE_KEY) ?? "";
  });

  const setToken = useCallback((value: string) => {
    setTokenState(value);
    if (typeof window === "undefined") {
      return;
    }
    if (value) {
      window.localStorage.setItem(STORAGE_KEY, value);
    } else {
      window.localStorage.removeItem(STORAGE_KEY);
    }
  }, []);

  const clearToken = useCallback(() => {
    setToken("");
  }, [setToken]);

  return [token, setToken, clearToken];
}
