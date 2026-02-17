import { formatName } from "@test/utils";

export interface User {
  name: string;
  email: string;
}

export function createUser(first: string, last: string, email: string): User {
  const name = formatName(first, last);
  return { name, email };
}

export function greet(user: User): string {
  return `Hello, ${user.name}`;
}
