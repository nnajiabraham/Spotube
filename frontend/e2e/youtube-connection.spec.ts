import { test, expect } from '@playwright/test';

test.describe('YouTube Connection Flow', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the dashboard
    await page.goto('/dashboard');
  });

  test('displays YouTube connection card on dashboard', async ({ page }) => {
    // Check that the YouTube connection card is visible
    const youtubeCard = page.locator('text=Connect YouTube').first();
    await expect(youtubeCard).toBeVisible();
    
    // Check that the card contains the expected text
    await expect(page.locator('text=Connect your YouTube account to start syncing your playlists.')).toBeVisible();
  });

  test('redirects to Google OAuth when clicking connect', async ({ page }) => {
    // Find and click the Connect YouTube button/link
    const connectButton = page.locator('a[href="/api/auth/google/login"]');
    await expect(connectButton).toBeVisible();
    
    // Intercept the navigation to the OAuth endpoint
    const [response] = await Promise.all([
      page.waitForResponse('/api/auth/google/login'),
      connectButton.click()
    ]);
    
    // Should redirect to Google OAuth
    expect(response.status()).toBe(307); // Temporary redirect
    expect(response.headers()['location']).toContain('accounts.google.com');
  });

  test('shows connected state after successful OAuth', async ({ page }) => {
    // Mock the API to return connected state
    await page.route('/api/youtube/playlists', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [
            {
              id: 'PLtest123',
              title: 'My Test Playlist',
              itemCount: 10,
              description: 'A test playlist'
            }
          ]
        })
      });
    });

    // Navigate to dashboard with youtube=connected query param (simulating callback)
    await page.goto('/dashboard?youtube=connected');
    
    // Check for success toast
    await expect(page.locator('text=YouTube connected successfully')).toBeVisible();
    
    // Reload to see the connected state
    await page.reload();
    
    // Check that the card now shows connected state
    await expect(page.locator('text=YouTube Connected')).toBeVisible();
    await expect(page.locator('text=Your YouTube account is connected and ready to sync.')).toBeVisible();
    
    // Check for View Playlists link
    const viewPlaylistsLink = page.locator('a[href="/settings/youtube"]');
    await expect(viewPlaylistsLink).toBeVisible();
  });

  test('displays YouTube playlists on settings page', async ({ page }) => {
    // Mock the API to return playlists
    await page.route('/api/youtube/playlists', async route => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          items: [
            {
              id: 'PLtest123',
              title: 'My Music Playlist',
              itemCount: 25,
              description: 'Collection of favorite songs'
            },
            {
              id: 'PLtest456',
              title: 'Workout Mix',
              itemCount: 15,
              description: 'High energy workout music'
            }
          ]
        })
      });
    });

    // Navigate to YouTube settings page
    await page.goto('/settings/youtube');
    
    // Check page title
    await expect(page.locator('h1:has-text("YouTube Playlists")')).toBeVisible();
    
    // Check that playlists are displayed
    await expect(page.locator('text=My Music Playlist')).toBeVisible();
    await expect(page.locator('text=25 tracks')).toBeVisible();
    await expect(page.locator('text=Workout Mix')).toBeVisible();
    await expect(page.locator('text=15 tracks')).toBeVisible();
  });

  test('handles OAuth error gracefully', async ({ page }) => {
    // Navigate to dashboard with error query param
    await page.goto('/dashboard?youtube=error&message=access_denied');
    
    // Check for error toast
    await expect(page.locator('text=Failed to connect YouTube: access_denied')).toBeVisible();
    
    // Card should still show disconnected state
    await expect(page.locator('text=Connect YouTube')).toBeVisible();
  });
}); 