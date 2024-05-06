import { z } from "zod"
import { extendZodWithOpenApi } from '@asteasolutions/zod-to-openapi';
import { Quad } from "n3";
import { isString, quote } from "./utils";

extendZodWithOpenApi(z);

// KeyPartのスキーマ
const StringPartSchema = z.object({
    type: z.literal('Key').or(z.literal('Literal')),
    value: z.string(),
});

// CommandStringのスキーマ
// LiteralPartまたはKeyPartのいずれかを要素とする配列
export const CommandStringSchema = z.array(StringPartSchema).openapi('CommandString');
type StringPart = z.infer<typeof StringPartSchema>
export type CommandString = (StringPart)[];

export function CommandStringToString(str:CommandString):string{
    return str.map((p)=>{
        if (p.type === 'Literal'){
            return p.value
        }else if(p.type === 'Key'){
            return `${p.value}`
        }
    }).join()
}

export function toCommandStringArray(str:(string|CommandString)[]):CommandString[]{
    return str.map((p)=>{
        if(isString(p)){
            return [{type:'Literal',value:p}]
        }else{
            return p
        }
    })
}

export function quoteCommand(s: CommandString): CommandString {
    return s.map((p)=>{
        if(p.type === 'Literal'){
            return {type:"Literal",value:quote(p.value)}
        }
        return p
    })
}