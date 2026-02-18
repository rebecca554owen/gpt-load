import { ref } from "vue";

export interface TestResult {
  duration: number;
  timestamp: number;
}

export function useModelTestStatus() {
  const testingModels = ref<Set<string>>(new Set());
  const successfullyTestedModels = ref<Set<string>>(new Set());
  const failedTestedModels = ref<Set<string>>(new Set());
  const testResults = ref<Map<string, TestResult>>(new Map());

  function startTesting(testKey: string): boolean {
    if (testingModels.value.has(testKey)) {
      return false;
    }
    testingModels.value.add(testKey);
    return true;
  }

  function setSuccess(testKey: string, duration: number): void {
    successfullyTestedModels.value.add(testKey);
    failedTestedModels.value.delete(testKey);
    testResults.value.set(testKey, {
      duration: duration || 0,
      timestamp: Date.now(),
    });
  }

  function setFailure(testKey: string): void {
    failedTestedModels.value.add(testKey);
    successfullyTestedModels.value.delete(testKey);
    testResults.value.delete(testKey);
  }

  function finishTesting(testKey: string): void {
    testingModels.value.delete(testKey);
  }

  function clearAll(): void {
    successfullyTestedModels.value.clear();
    failedTestedModels.value.clear();
    testResults.value.clear();
  }

  function isTesting(testKey: string): boolean {
    return testingModels.value.has(testKey);
  }

  function hasFailed(testKey: string): boolean {
    return failedTestedModels.value.has(testKey);
  }

  function hasSucceeded(testKey: string): boolean {
    return successfullyTestedModels.value.has(testKey);
  }

  function getResult(testKey: string): TestResult | undefined {
    return testResults.value.get(testKey);
  }

  return {
    testingModels,
    successfullyTestedModels,
    failedTestedModels,
    testResults,
    startTesting,
    setSuccess,
    setFailure,
    finishTesting,
    clearAll,
    isTesting,
    hasFailed,
    hasSucceeded,
    getResult,
  };
}

export type UseModelTestStatusReturn = ReturnType<typeof useModelTestStatus>;
