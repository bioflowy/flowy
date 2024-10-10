
import { describe, it } from 'vitest';
import * as cwlTsAuto from '@flowy/cwl-ts-auto';
// 再帰的に探索して特定のクラスのインスタンスを抽出する関数
function extractInstances<T>(
    data: any,
    targetClass: new (...args: any[]) => T
  ): T[] {
    const results: T[] = [];
  
    function traverse(item: any) {
      if (item instanceof targetClass) {
        results.push(item);
      } else if (Array.isArray(item)) {
        item.forEach(traverse);
      } else if (typeof item === 'object' && item !== null) {
        Object.values(item).forEach(traverse);
      }
    }
  
    traverse(data);
    return results;
  }
describe('JavaScript Engine Evaluation',() => {
  it('evaluates simple arithmetic expression "1+1"',async  () => {
    const loadingOptions = new cwlTsAuto.LoadingOptions({});
    const doc = await cwlTsAuto.loadDocument("/home/uehara/flowy/cwl-v1.2-main/tests/search.cwl", "file:///home/uehara/flowy/cwl-v1.2-main/tests/search.cwl", loadingOptions);
    const files = extractInstances(doc, cwlTsAuto.File)
    console.log(files)
  });
});