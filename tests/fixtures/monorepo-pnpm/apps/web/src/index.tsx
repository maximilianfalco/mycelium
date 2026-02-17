import { createUser, greet } from "@test/core";
import { add } from "@test/utils";
import { validateUser } from "@test/core/src/validator";
import React from "react";
import type { User } from "@test/core";

export default function App(): JSX.Element {
  const user = createUser("John", "Doe", "john@example.com");
  const message = greet(user);
  const sum = add(1, 2);
  const isValid = validateUser(user);
  return (
    <div>
      {message} {sum} {isValid}
    </div>
  );
}
