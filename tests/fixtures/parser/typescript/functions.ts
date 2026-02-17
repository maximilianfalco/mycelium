/** Adds two numbers together */
function add(a: number, b: number): number {
  return a + b;
}

export function greet(name: string): string {
  return `Hello, ${name}!`;
}

const multiply = (a: number, b: number): number => {
  return a * b;
}

const double = (n: number): number => n * 2;

async function fetchData(url: string): Promise<Response> {
  return fetch(url);
}
