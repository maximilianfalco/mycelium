export default function defaultHandler(req: Request): Response {
  return new Response("ok");
}

// Overloaded function
function format(value: string): string;
function format(value: number): string;
function format(value: string | number): string {
  return String(value);
}

const asyncArrow = async (url: string): Promise<void> => {
  await fetch(url);
};

abstract class BaseService {
  abstract execute(): void;

  log(msg: string): void {
    console.log(msg);
  }
}

function outer(): void {
  const inner = (x: number) => x + 1;
  console.log(inner(1));
}
