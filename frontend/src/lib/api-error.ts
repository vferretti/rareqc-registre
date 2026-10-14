import { isAxiosError } from "axios";

/** True when the failed request was rejected with 409 Conflict (unique-value collision). */
export function isApiConflict(err: unknown): boolean {
  return isAxiosError(err) && err.response?.status === 409;
}
