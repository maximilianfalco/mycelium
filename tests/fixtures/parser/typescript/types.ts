type Status = "active" | "inactive" | "pending";

type Pair<A, B> = {
  first: A;
  second: B;
};

enum Direction {
  Up = "UP",
  Down = "DOWN",
  Left = "LEFT",
  Right = "RIGHT",
}

enum Color {
  Red,
  Green,
  Blue,
}
