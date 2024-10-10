import { serve } from '@hono/node-server'
import app from './app'
import yargs, { config } from 'yargs';
import { hideBin } from 'yargs/helpers';
import { initManager } from './server/manager';

interface Args {
  config?: string;
  port: number;
}
function main(args: Args){
  console.log(`loading config file ${args.config}`)
  initManager(args.config)
  console.log(`Server is running on port ${args.port}`)
  serve({
    fetch: app.fetch,
    port: args.port
  })
  
}
const arg = yargs(hideBin(process.argv))
.command('flowy-manager', 'execute cwl workflow')
.option('port', {
  alias: 'p',
  description: 'port number',
  type: 'number',
  default: 5173,
})
.option('config', {
  alias: 'c',
  description: 'config file path',
  type: 'string',
})
.help()
.parseSync();

main(arg as Args)