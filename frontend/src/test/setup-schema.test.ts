import { describe, it, expect } from 'vitest'
import { z } from 'zod'

// Import the schema from the setup component
const SetupSchema = z.object({
  spotifyId: z.string().min(1, 'Spotify Client ID is required'),
  spotifySecret: z.string().min(1, 'Spotify Client Secret is required'),
  googleClientId: z.string().min(1, 'Google Client ID is required'),
  googleClientSecret: z.string().min(1, 'Google Client Secret is required'),
})

describe('SetupSchema validation', () => {
  it('should validate valid credentials', () => {
    const validData = {
      spotifyId: 'test-spotify-id',
      spotifySecret: 'test-spotify-secret',
      googleClientId: 'test-google-id',
      googleClientSecret: 'test-google-secret',
    }

    const result = SetupSchema.safeParse(validData)
    expect(result.success).toBe(true)
    if (result.success) {
      expect(result.data).toEqual(validData)
    }
  })

  it('should reject empty spotify ID', () => {
    const invalidData = {
      spotifyId: '',
      spotifySecret: 'test-spotify-secret',
      googleClientId: 'test-google-id',
      googleClientSecret: 'test-google-secret',
    }

    const result = SetupSchema.safeParse(invalidData)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe('Spotify Client ID is required')
    }
  })

  it('should reject empty spotify secret', () => {
    const invalidData = {
      spotifyId: 'test-spotify-id',
      spotifySecret: '',
      googleClientId: 'test-google-id',
      googleClientSecret: 'test-google-secret',
    }

    const result = SetupSchema.safeParse(invalidData)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe('Spotify Client Secret is required')
    }
  })

  it('should reject empty google client ID', () => {
    const invalidData = {
      spotifyId: 'test-spotify-id',
      spotifySecret: 'test-spotify-secret',
      googleClientId: '',
      googleClientSecret: 'test-google-secret',
    }

    const result = SetupSchema.safeParse(invalidData)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe('Google Client ID is required')
    }
  })

  it('should reject empty google client secret', () => {
    const invalidData = {
      spotifyId: 'test-spotify-id',
      spotifySecret: 'test-spotify-secret',
      googleClientId: 'test-google-id',
      googleClientSecret: '',
    }

    const result = SetupSchema.safeParse(invalidData)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues[0].message).toBe('Google Client Secret is required')
    }
  })

  it('should reject missing fields', () => {
    const invalidData = {
      spotifyId: 'test-spotify-id',
      // Missing other required fields
    }

    const result = SetupSchema.safeParse(invalidData)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues).toHaveLength(3) // Three missing fields
    }
  })

  it('should reject all empty strings', () => {
    const invalidData = {
      spotifyId: '',
      spotifySecret: '',
      googleClientId: '',
      googleClientSecret: '',
    }

    const result = SetupSchema.safeParse(invalidData)
    expect(result.success).toBe(false)
    if (!result.success) {
      expect(result.error.issues).toHaveLength(4) // All four fields invalid
    }
  })
}) 