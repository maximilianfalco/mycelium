interface Printable {
  print(): void;
}

interface Repository<T> {
  findById(id: string): T | undefined;
  save(item: T): void;
}

interface CrudRepository<T> extends Repository<T> {
  delete(id: string): boolean;
}
