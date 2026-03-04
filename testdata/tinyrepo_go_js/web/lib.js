import fs from 'fs'

export function greet(name) {
  // Touch a Node builtin so we have an external import target.
  if (fs.existsSync('.')) {
    return `hello ${name}`
  }
  return `hello ${name}`
}

