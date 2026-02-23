import http from "@/utils/http";
import { useAuthStore } from "@/stores/auth";

export function useAuthKey() {
  const authStore = useAuthStore();
  return {
    get value() {
      return authStore.authKey;
    },
    set value(key: string | null) {
      authStore.setAuthKey(key);
    },
  };
}

export function useAuthService() {
  const authKey = useAuthKey();
  const authStore = useAuthStore();

  const login = async (key: string): Promise<boolean> => {
    try {
      await http.post("/auth/login", { auth_key: key });
      authStore.setAuthKey(key);
      return true;
    } catch (_error) {
      return false;
    }
  };

  const logout = (): void => {
    authStore.clearAuthKey();
  };

  const checkLogin = (): boolean => {
    if (authKey.value) {
      return true;
    }

    const key = localStorage.getItem("authKey");
    if (key) {
      authStore.setAuthKey(key);
    }
    return !!authKey.value;
  };

  return {
    login,
    logout,
    checkLogin,
  };
}
