//import HelloWorld from './HelloWorld'
import {
  dateEndFormat,
  dateFormat,
  formatLongDate,
  getTestDetailsLink,
} from './CompReadyUtils'

test('parses a query param start time without timezone', () => {
  let result = formatLongDate('2024-09-05 00:00:00', dateFormat)
  expect(result).toBe('2024-09-05 00:00:00')
})

test('parses a query param start time eod without timezone', () => {
  let result = formatLongDate('2024-09-05 23:59:59', dateFormat)
  expect(result).toBe('2024-09-05 00:00:00')
})

test('parses a query param end time without timezone', () => {
  let result = formatLongDate('2024-09-05 23:59:59', dateEndFormat)
  expect(result).toBe('2024-09-05 23:59:59')
})

test('parses an ISO8601 start time', () => {
  let result = formatLongDate('2024-08-06T00:00:00Z', dateFormat)
  expect(result).toBe('2024-08-06 00:00:00')
})

test('parses an ISO8601 end time mid-day', () => {
  let result = formatLongDate('2024-08-06T08:00:00Z', dateEndFormat)
  expect(result).toBe('2024-08-06 23:59:59')
})

test('parses an ISO8601 end time', () => {
  let result = formatLongDate('2024-08-06T23:59:59Z', dateEndFormat)
  expect(result).toBe('2024-08-06 23:59:59')
})

describe('getTestDetailsLink', () => {
  test('returns null for null links', () => {
    expect(getTestDetailsLink(null)).toBeNull()
  })

  test('returns plain test_details key when present', () => {
    const links = { test_details: '/api/test/details' }
    expect(getTestDetailsLink(links)).toBe('/api/test/details')
  })

  test('returns exact view key when viewName matches', () => {
    const links = {
      'test_details:4.22-main': '/api/main',
      'test_details:4.22-arm64': '/api/arm64',
    }
    expect(getTestDetailsLink(links, '4.22-arm64')).toBe('/api/arm64')
  })

  test('returns null when viewName is specified but key is missing', () => {
    const links = {
      'test_details:4.22-main': '/api/main',
    }
    expect(getTestDetailsLink(links, '4.22-arm64')).toBeNull()
  })

  test('falls back to first test_details: key when no viewName', () => {
    const links = {
      'test_details:4.22-main': '/api/main',
    }
    expect(getTestDetailsLink(links)).toBe('/api/main')
  })

  test('returns null when no matching keys exist', () => {
    const links = { self: '/api/self' }
    expect(getTestDetailsLink(links)).toBeNull()
  })

  test('plain test_details takes priority over view-specific keys', () => {
    const links = {
      test_details: '/api/plain',
      'test_details:4.22-main': '/api/main',
    }
    expect(getTestDetailsLink(links, '4.22-main')).toBe('/api/plain')
    expect(getTestDetailsLink(links)).toBe('/api/plain')
  })
})
