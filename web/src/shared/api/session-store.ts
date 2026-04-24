import { create } from "zustand";

type SessionState = {
  adminKey: string;
  setAdminKey: (value: string) => void;
};

const storageKey = "aiclient-kiro-admin-key";

export const useSessionStore = create<SessionState>((set) => ({
  adminKey: typeof localStorage === "undefined" ? "" : localStorage.getItem(storageKey) ?? "",
  setAdminKey: (value) => {
    if (value) {
      localStorage.setItem(storageKey, value);
    } else {
      localStorage.removeItem(storageKey);
    }
    set({ adminKey: value });
  },
}));
