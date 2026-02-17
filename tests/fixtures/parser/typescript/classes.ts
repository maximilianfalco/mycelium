/** A shape with area calculation */
class Shape {
  constructor(public name: string) {}

  area(): number {
    return 0;
  }
}

class Circle extends Shape {
  constructor(public radius: number) {
    super("circle");
  }

  area(): number {
    return Math.PI * this.radius ** 2;
  }

  static unit(): Circle {
    return new Circle(1);
  }
}
