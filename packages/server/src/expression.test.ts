import { describe, it, expect } from 'vitest';
import { get_js_engine } from './sandbox';
import { do_eval } from './expression';

describe('expression Evaluation',() => {
  it('expression eval with inputs,resources',async  () => {
    const result = await do_eval("inputs.test=$(inputs.test),cores=$(runtime.cores)",{"test":123},undefined,"/tmp/outdir","/tmp/tmpdir-123",{"cores":2},{})
    expect(result).toBe("inputs.test=123,cores=2");
  });
});