import { greet } from './lib.js'
import lodash from 'lodash'

export function main() {
  return greet(lodash.capitalize('falcon'))
}

