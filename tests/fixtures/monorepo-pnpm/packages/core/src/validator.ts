import { User } from "./index";

export function validateUser(user: User): boolean {
  return user.name.length > 0 && user.email.includes("@");
}
