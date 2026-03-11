import { greet } from './lib.js'
import lodash from 'lodash'
export { greet } from './lib.js'

export function main() {
  return greet(lodash.capitalize('falcon'))
}

