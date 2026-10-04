// SPDX-License-Identifier: AGPL-3.0-or-later
// Serialize only native outcome and completion events. Error properties are not enumerable.
export default async function* (source) {
  for await (const event of source) {
    if (['test:pass', 'test:fail', 'test:summary'].includes(event.type)) {
      yield JSON.stringify(event, (_, value) => value instanceof Error
        ? { message: value.message, code: value.code, failureType: value.failureType }
        : value) + '\n';
    }
  }
}
