
import * as Internal from './util/Internal.js'


/**
 * Auto-generated interface for https://w3id.org/cwl/cwl#EnvironmentDef
 *
 * Define an environment variable that will be set in the runtime environment
 * by the workflow platform when executing the command line tool.  May be the
 * result of executing an expression, such as getting a parameter from input.
 * 
 */
export interface EnvironmentDefProperties  {
                    
  extensionFields?: Internal.Dictionary<any>

  /**
   * The environment variable name
   */
  envName: string

  /**
   * The environment variable value
   */
  envValue: string
}