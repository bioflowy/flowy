import { parse } from 'uri-js';

const result = parse('reference');

console.log(result);

const result2 = new URL('reference');

console.log(result2)
