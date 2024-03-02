import { describe, it, expect } from 'vitest';
import { get_js_engine } from './sandbox';

describe('JavaScript Engine Evaluation',() => {
  it('evaluates simple arithmetic expression "1+1"',async  () => {
    const js_engine = get_js_engine()
    const rslt = await js_engine.eval("1+1","",{})
    expect(rslt).toBe(2);
  });
  it('evaluates with rootvars',async  () => {
    const js_engine = get_js_engine()
    const rslt = await js_engine.eval("test + cores","",{"test":123,"cores":2})
    expect(rslt).toBe(125);
  })
});