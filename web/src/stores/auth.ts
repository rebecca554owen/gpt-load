import { defineStore } from "pinia";

export const useAuthStore = defineStore("auth", {
  state: () => ({
    authKey: localStorage.getItem("authKey") as string | null,
  }),

  actions: {
    setAuthKey(key: string | null) {
      this.authKey = key;
      if (key) {
        localStorage.setItem("authKey", key);
      } else {
        localStorage.removeItem("authKey");
      }
    },

    clearAuthKey() {
      this.authKey = null;
      localStorage.removeItem("authKey");
    },
  },
});
