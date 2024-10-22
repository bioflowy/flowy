
import { describe, it } from 'vitest';
import * as cwlTsAuto from '@flowy/cwl-ts-auto';
describe('JavaScript Engine Evaluation',() => {
  it('evaluates simple arithmetic expression "1+1"',async  () => {
    const loadingOptions = new cwlTsAuto.LoadingOptions({});
    const doc = await cwlTsAuto.loadDocument("/home/uehara/flowy/cwl-v1.2-main/tests/search.cwl", "file:///home/uehara/flowy/cwl-v1.2-main/tests/search.cwl", loadingOptions);
    const files = extractInstances(doc, cwlTsAuto.File)
    console.log(files)
  });
});