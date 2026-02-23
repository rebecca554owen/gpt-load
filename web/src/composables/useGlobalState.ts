import { isRef, reactive, toRef, type Ref } from "vue";

type IntializeFunc<T> = () => T | Ref<T>;
type InitializeValue<T> = T | Ref<T> | IntializeFunc<T>;
type GlobalState = Record<string, unknown>;

const globalState = reactive<GlobalState>({});

export function useState<T>(key: string, init?: InitializeValue<T>): Ref<T> {
  const state = toRef(globalState, key);

  if (state.value === undefined && init !== undefined) {
    const initialValue = init instanceof Function ? init() : init;

    if (isRef(initialValue)) {
      globalState[key] = initialValue;

      return initialValue;
    }

    state.value = initialValue as T;
  }

  return state as Ref<T>;
}
