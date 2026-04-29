import { create } from "zustand";

type SessionState = {
  adminKey: string;
  rememberAdminKey: boolean;
  setAdminKey: (value: string, options?: { remember?: boolean }) => void;
  clearAdminKey: () => void;
};

const storageKey = "orik-admin-key";

function readStoredKey() {
  if (typeof window === "undefined") {
    return { key: "", remember: false };
  }
  const sessionKey = window.sessionStorage.getItem(storageKey);
  if (sessionKey) {
    return { key: sessionKey, remember: false };
  }
  return { key: window.localStorage.getItem(storageKey) ?? "", remember: Boolean(window.localStorage.getItem(storageKey)) };
}

const stored = readStoredKey();

export const useSessionStore = create<SessionState>((set) => ({
  adminKey: stored.key,
  rememberAdminKey: stored.remember,
  setAdminKey: (value, options) => {
    const remember = Boolean(options?.remember);
    if (value) {
      if (remember) {
        window.localStorage.setItem(storageKey, value);
        window.sessionStorage.removeItem(storageKey);
      } else {
        window.sessionStorage.setItem(storageKey, value);
        window.localStorage.removeItem(storageKey);
      }
    } else {
      window.sessionStorage.removeItem(storageKey);
      window.localStorage.removeItem(storageKey);
    }
    set({ adminKey: value, rememberAdminKey: remember });
  },
  clearAdminKey: () => {
    window.sessionStorage.removeItem(storageKey);
    window.localStorage.removeItem(storageKey);
    set({ adminKey: "", rememberAdminKey: false });
  },
}));
