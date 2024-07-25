
export class AsyncFIFOQueue<T> {
    private queue: T[] = [];
    private waiters: ((value: T | null) => void)[] = [];
  
    push(item: T): void {
      if (this.waiters.length > 0) {
        const waiter = this.waiters.shift()!;
        waiter(item);
      } else {
        this.queue.push(item);
      }
    }
  
    async pop(timeout: number = 0): Promise<T | null> {
      if (this.queue.length > 0) {
        return this.queue.shift()!;
      }
  
      return new Promise<T | null>((resolve) => {
        const waiter = (value: T | null) => resolve(value);
        this.waiters.push(waiter);
  
        if (timeout > 0) {
          setTimeout(() => {
            const index = this.waiters.indexOf(waiter);
            if (index !== -1) {
              this.waiters.splice(index, 1);
              resolve(null);
            }
          }, timeout);
        }
      });
    }
  
    size(): number {
      return this.queue.length;
    }
  }