"use strict";
process.stdin.setEncoding("utf8");
var incoming = "";
process.stdin.on("data", function(chunk) {
  incoming += chunk;
  var i = incoming.indexOf("\n");
  if (i > -1) {
    try{
      var fn = JSON.parse(incoming.substr(0, i));
      incoming = incoming.substr(i+1);
      const str = JSON.stringify(require("vm").runInNewContext(fn, {})) + "\n"
      process.stderr.write(`strlen=${str.length}\n`);
      process.stdout.write(str,(err) => {
        process.exit(0);
      });
      process.stdout.end();
    }
    catch(e){
      console.error(e)
      process.exit(1);
    }
  }
});
