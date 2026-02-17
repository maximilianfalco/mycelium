// @ts-nocheck
import { validateToken, hashPassword } from "@company/auth";
import * as utils from "./utils";
import React from "react";
import type { UserType } from "./types";

interface Serializable {
  serialize(): string;
}

interface Loggable {
  log(msg: string): void;
}

type UserId = string;

class User {
  name: string;
  id: UserId;

  constructor(name: string) {
    this.name = name;
    this.id = hashPassword(name);
  }

  greet(): string {
    return `Hello, ${this.name}`;
  }
}

class Admin extends User implements Serializable, Loggable {
  role: string;

  constructor(name: string, role: string) {
    super(name);
    this.role = role;
    this.init();
  }

  serialize(): string {
    return JSON.stringify(this);
  }

  log(msg: string): void {
    console.log(msg);
  }

  promote(target: User): void {
    validateToken(target.name);
    utils.notify(target);
  }
}

function processUser(user: User): Promise<UserType> {
  const token = validateToken(user.name);
  const hash = hashPassword(token);
  console.log(hash);
  return Promise.resolve(user as unknown as UserType);
}

const createAdmin = (name: string): Admin => {
  return new Admin(name, "superadmin");
};
