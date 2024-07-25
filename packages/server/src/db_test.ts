import { v4 } from "uuid";
import { db } from "./databases";

async function insertInto(){
    await db.insertInto("job").values({
        id: v4(),
        name: "test_db_insert",
        status: "Created",
        type:"CommandLine",
        inputs: '{"test":123}'
    }).execute()
}

insertInto().then(()=>{
    console.log("finished")
}).catch((e)=>[
    console.log(e)
])