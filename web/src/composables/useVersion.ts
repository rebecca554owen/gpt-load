import axios from "axios";
import { ref, type Ref } from "vue";

export interface GitHubRelease {
  tag_name: string;
  html_url: string;
  published_at: string;
  name: string;
}

export interface VersionInfo {
  currentVersion: string;
  latestVersion: string | null;
  isLatest: boolean;
  hasUpdate: boolean;
  releaseUrl: string | null;
  lastCheckTime: number;
  status: "checking" | "latest" | "update-available" | "error";
}

const CACHE_KEY = "gpt-load-version-info";
const CACHE_DURATION = 30 * 60 * 1000;

const currentVersion = import.meta.env.VITE_VERSION || "1.0.0";

function getCachedVersionInfo(): VersionInfo | null {
  try {
    const cached = localStorage.getItem(CACHE_KEY);
    if (!cached) {
      return null;
    }

    const versionInfo: VersionInfo = JSON.parse(cached);
    const now = Date.now();

    if (now - versionInfo.lastCheckTime > CACHE_DURATION) {
      return null;
    }

    if (versionInfo.currentVersion !== currentVersion) {
      clearCache();
      return null;
    }

    return versionInfo;
  } catch (error) {
    console.warn("Failed to parse cached version info:", error);
    localStorage.removeItem(CACHE_KEY);
    return null;
  }
}

function setCachedVersionInfo(versionInfo: VersionInfo): void {
  try {
    localStorage.setItem(CACHE_KEY, JSON.stringify(versionInfo));
  } catch (error) {
    console.warn("Failed to cache version info:", error);
  }
}

function compareVersions(current: string, latest: string): number {
  const currentParts = current.replace(/^v/, "").split(".").map(Number);
  const latestParts = latest.replace(/^v/, "").split(".").map(Number);

  for (let i = 0; i < Math.max(currentParts.length, latestParts.length); i++) {
    const currentPart = currentParts[i] || 0;
    const latestPart = latestParts[i] || 0;

    if (currentPart < latestPart) {
      return -1;
    }
    if (currentPart > latestPart) {
      return 1;
    }
  }

  return 0;
}

async function fetchLatestVersion(): Promise<GitHubRelease | null> {
  try {
    const response = await axios.get(
      "https://api.github.com/repos/tbphp/gpt-load/releases/latest",
      {
        timeout: 10000,
        headers: {
          Accept: "application/vnd.github.v3+json",
        },
      }
    );

    if (response.status === 200 && response.data) {
      return response.data;
    }

    return null;
  } catch (error) {
    console.warn("Failed to fetch latest version from GitHub:", error);
    return null;
  }
}

async function checkForUpdates(): Promise<VersionInfo> {
  const cached = getCachedVersionInfo();
  if (cached) {
    return cached;
  }

  const versionInfo: VersionInfo = {
    currentVersion,
    latestVersion: null,
    isLatest: false,
    hasUpdate: false,
    releaseUrl: null,
    lastCheckTime: Date.now(),
    status: "checking",
  };

  try {
    const release = await fetchLatestVersion();

    if (release) {
      const comparison = compareVersions(currentVersion, release.tag_name);

      versionInfo.latestVersion = release.tag_name;
      versionInfo.releaseUrl = release.html_url;
      versionInfo.isLatest = comparison >= 0;
      versionInfo.hasUpdate = comparison < 0;
      versionInfo.status = comparison < 0 ? "update-available" : "latest";

      setCachedVersionInfo(versionInfo);
    } else {
      versionInfo.status = "error";
    }
  } catch (error) {
    console.warn("Version check failed:", error);
    versionInfo.status = "error";
  }

  return versionInfo;
}

function getCurrentVersion(): string {
  return currentVersion;
}

function clearCache(): void {
  localStorage.removeItem(CACHE_KEY);
}

export function useVersion() {
  const versionInfo = ref<VersionInfo>({
    currentVersion,
    latestVersion: null,
    isLatest: false,
    hasUpdate: false,
    releaseUrl: null,
    lastCheckTime: 0,
    status: "checking",
  }) as Ref<VersionInfo>;

  const isChecking = ref(false);

  const checkVersion = async () => {
    if (isChecking.value) {
      return;
    }

    isChecking.value = true;
    try {
      const result = await checkForUpdates();
      versionInfo.value = result;
    } catch (error) {
      console.warn("Version check failed:", error);
    } finally {
      isChecking.value = false;
    }
  };

  return {
    versionInfo,
    isChecking,
    checkVersion,
    getCurrentVersion,
    clearCache,
  };
}
