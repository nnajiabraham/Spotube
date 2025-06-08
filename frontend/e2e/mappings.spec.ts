import { test, expect } from '@playwright/test'

test.describe('Playlist Mappings', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the dashboard
    await page.goto('/dashboard')
  })

  test('shows mappings card on dashboard', async ({ page }) => {
    // Look for the mappings card
    const mappingsCard = page.locator('text=Playlist Mappings')
    await expect(mappingsCard).toBeVisible()
    
    // Check for the "View Mappings" button
    const viewMappingsButton = page.locator('a[href="/mappings"]')
    await expect(viewMappingsButton).toHaveText('View Mappings')
  })

  test('navigates to mappings list', async ({ page }) => {
    // Click on View Mappings
    await page.click('a[href="/mappings"]')
    
    // Should be on mappings page
    await expect(page).toHaveURL('/mappings')
    
    // Should see the mappings title
    await expect(page.locator('h1')).toHaveText('Playlist Mappings')
  })

  test('shows empty state when no mappings exist', async ({ page }) => {
    // Mock empty mappings response
    await page.route('**/api/collections/mappings/records', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          page: 1,
          perPage: 30,
          totalItems: 0,
          totalPages: 0,
          items: [],
        }),
      })
    })

    await page.goto('/mappings')
    
    // Should show empty state message
    await expect(page.locator('text=No mappings yet')).toBeVisible()
  })

  test('displays existing mappings in table', async ({ page }) => {
    // Mock mappings response
    await page.route('**/api/collections/mappings/records', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          page: 1,
          perPage: 30,
          totalItems: 1,
          totalPages: 1,
          items: [{
            id: 'test-mapping',
            spotify_playlist_id: 'spotify123',
            youtube_playlist_id: 'youtube456',
            spotify_playlist_name: 'Test Spotify Playlist',
            youtube_playlist_name: 'Test YouTube Playlist',
            sync_name: true,
            sync_tracks: true,
            interval_minutes: 60,
            created: '2024-01-01T00:00:00Z',
            updated: '2024-01-01T00:00:00Z',
          }],
        }),
      })
    })

    await page.goto('/mappings')
    
    // Should show the mapping in the table
    await expect(page.locator('text=Test Spotify Playlist')).toBeVisible()
    await expect(page.locator('text=Test YouTube Playlist')).toBeVisible()
    await expect(page.locator('text=✓ Name')).toBeVisible()
    await expect(page.locator('text=✓ Tracks')).toBeVisible()
    await expect(page.locator('text=60 min')).toBeVisible()
  })

  test('can navigate to create new mapping', async ({ page }) => {
    await page.goto('/mappings')
    
    // Click the Add mapping button
    await page.click('text=Add mapping')
    
    // Should navigate to new mapping page
    await expect(page).toHaveURL('/mappings/new')
    await expect(page.locator('h1')).toHaveText('Create New Mapping')
  })

  test('shows delete confirmation dialog', async ({ page }) => {
    // Mock mappings response
    await page.route('**/api/collections/mappings/records', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          page: 1,
          perPage: 30,
          totalItems: 1,
          totalPages: 1,
          items: [{
            id: 'test-mapping',
            spotify_playlist_id: 'spotify123',
            youtube_playlist_id: 'youtube456',
            spotify_playlist_name: 'Test Playlist',
            youtube_playlist_name: 'Test YT Playlist',
            sync_name: true,
            sync_tracks: true,
            interval_minutes: 60,
            created: '2024-01-01T00:00:00Z',
            updated: '2024-01-01T00:00:00Z',
          }],
        }),
      })
    })

    await page.goto('/mappings')
    
    // Set up dialog handler
    page.once('dialog', dialog => {
      expect(dialog.message()).toBe('Are you sure you want to delete this mapping?')
      dialog.dismiss() // Cancel the deletion
    })
    
    // Click delete button
    await page.click('button.text-red-600')
  })

  test('can navigate to edit mapping', async ({ page }) => {
    // Mock mappings response
    await page.route('**/api/collections/mappings/records', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          page: 1,
          perPage: 30,
          totalItems: 1,
          totalPages: 1,
          items: [{
            id: 'test-mapping-123',
            spotify_playlist_id: 'spotify123',
            youtube_playlist_id: 'youtube456',
            spotify_playlist_name: 'Test Playlist',
            youtube_playlist_name: 'Test YT Playlist',
            sync_name: true,
            sync_tracks: true,
            interval_minutes: 60,
            created: '2024-01-01T00:00:00Z',
            updated: '2024-01-01T00:00:00Z',
          }],
        }),
      })
    })

    await page.goto('/mappings')
    
    // Click edit button
    await page.click('a[href="/mappings/test-mapping-123/edit"]')
    
    // Should navigate to edit page
    await expect(page).toHaveURL('/mappings/test-mapping-123/edit')
  })
}) 