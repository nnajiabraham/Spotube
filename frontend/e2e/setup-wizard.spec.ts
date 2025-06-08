import { test, expect } from '@playwright/test'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

interface SetupRequestBody {
  spotify_id: string
  spotify_secret: string
  google_client_id: string
  google_client_secret: string
}

// Setup MSW server
const server = setupServer(
  // Mock GET /api/setup/status - setup required
  http.get('/api/setup/status', () => {
    return HttpResponse.json({ required: true })
  }),

  // Mock POST /api/setup - successful save
  http.post('/api/setup', async ({ request }) => {
    const body = await request.json() as SetupRequestBody
    
    // Validate that all required fields are present
    if (
      !body.spotify_id ||
      !body.spotify_secret ||
      !body.google_client_id ||
      !body.google_client_secret
    ) {
      return new HttpResponse(
        'All credentials are required',
        { status: 400 }
      )
    }

    return new HttpResponse(null, { status: 204 })
  })
)

test.describe('Setup Wizard E2E', () => {
  test.beforeAll(async () => {
    server.listen()
  })

  test.afterEach(async () => {
    server.resetHandlers()
  })

  test.afterAll(async () => {
    server.close()
  })

  test('should display setup wizard when setup is required', async ({ page }) => {
    await page.goto('/')

    // Should redirect to setup page
    await expect(page).toHaveURL('/setup')
    
    // Should show the setup form
    await expect(page.locator('h2')).toContainText('Welcome to Spotube')
    await expect(page.locator('input[placeholder*="Spotify Client ID"]')).toBeVisible()
    await expect(page.locator('input[placeholder*="Spotify Client Secret"]')).toBeVisible()
    await expect(page.locator('input[placeholder*="Google Client ID"]')).toBeVisible()
    await expect(page.locator('input[placeholder*="Google Client Secret"]')).toBeVisible()
  })

  test('should validate required fields', async ({ page }) => {
    await page.goto('/setup')

    // Try to submit empty form
    await page.click('button[type="submit"]')

    // Should show validation errors
    await expect(page.locator('text=Spotify Client ID is required')).toBeVisible()
    await expect(page.locator('text=Spotify Client Secret is required')).toBeVisible()
    await expect(page.locator('text=Google Client ID is required')).toBeVisible()
    await expect(page.locator('text=Google Client Secret is required')).toBeVisible()
  })

  test('should successfully submit valid credentials', async ({ page }) => {
    await page.goto('/setup')

    // Fill in all required fields
    await page.fill('input[placeholder*="Spotify Client ID"]', 'test-spotify-id')
    await page.fill('input[placeholder*="Spotify Client Secret"]', 'test-spotify-secret')
    await page.fill('input[placeholder*="Google Client ID"]', 'test-google-id')
    await page.fill('input[placeholder*="Google Client Secret"]', 'test-google-secret')

    // Submit the form
    await page.click('button[type="submit"]')

    // Should redirect to success page
    await expect(page).toHaveURL('/setup/success')
    await expect(page.locator('h2')).toContainText('Setup Complete!')
  })

  test('should handle API errors gracefully', async ({ page }) => {
    // Mock API error response
    server.use(
      http.post('/api/setup', () => {
        return new HttpResponse(
          'Internal server error',
          { status: 500 }
        )
      })
    )

    await page.goto('/setup')

    // Fill in credentials
    await page.fill('input[placeholder*="Spotify Client ID"]', 'test-spotify-id')
    await page.fill('input[placeholder*="Spotify Client Secret"]', 'test-spotify-secret')
    await page.fill('input[placeholder*="Google Client ID"]', 'test-google-id')
    await page.fill('input[placeholder*="Google Client Secret"]', 'test-google-secret')

    // Submit the form
    await page.click('button[type="submit"]')

    // Should show error message
    await expect(page.locator('text=Error')).toBeVisible()
    await expect(page.locator('text=Internal server error')).toBeVisible()
  })

  test('should redirect to dashboard when setup is not required', async ({ page }) => {
    // Mock API to return setup not required
    server.use(
      http.get('/api/setup/status', () => {
        return HttpResponse.json({ required: false })
      })
    )

    await page.goto('/')

    // Should redirect to dashboard
    await expect(page).toHaveURL('/dashboard')
    await expect(page.locator('h1')).toContainText('Welcome to Spotube Dashboard')
  })

  test('should show loading state during form submission', async ({ page }) => {
    // Add delay to API response to test loading state
    server.use(
      http.post('/api/setup', async () => {
        await new Promise(resolve => setTimeout(resolve, 1000))
        return new HttpResponse(null, { status: 204 })
      })
    )

    await page.goto('/setup')

    // Fill in credentials
    await page.fill('input[placeholder*="Spotify Client ID"]', 'test-spotify-id')
    await page.fill('input[placeholder*="Spotify Client Secret"]', 'test-spotify-secret')
    await page.fill('input[placeholder*="Google Client ID"]', 'test-google-id')
    await page.fill('input[placeholder*="Google Client Secret"]', 'test-google-secret')

    // Submit the form
    await page.click('button[type="submit"]')

    // Should show loading state
    await expect(page.locator('button[disabled]')).toContainText('Saving...')
  })
}) 