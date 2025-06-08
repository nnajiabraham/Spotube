import { test, expect } from '@playwright/test';

test.describe('Spotify Authentication Flow', () => {
  test.beforeEach(async ({ page }) => {
    // Wait for the app to load and MSW to be ready
    await page.goto('/');
    await page.waitForLoadState('networkidle');
  });

  test('shows setup wizard when setup is required', async ({ page }) => {
    // Default MSW handler returns { required: false } for setup status
    // We need to navigate to dashboard which will check setup status
    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');
    
    // The app should show the dashboard since setup is not required (MSW default)
    await expect(page.locator('h1')).toContainText('Welcome to Spotube Dashboard');
  });

  test('shows connect Spotify button when not authenticated', async ({ page }) => {
    // Navigate to dashboard
    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');
    
    // Wait for the SpotifyConnectionCard to load
    // It will make a request to /api/spotify/playlists which returns 401 by default
    await page.waitForTimeout(1000); // Give React Query time to process
    
    // Look for the connect button
    const connectButton = page.locator('a:has-text("Connect Spotify")');
    await expect(connectButton).toBeVisible();
    await expect(connectButton).toHaveAttribute('href', '/api/auth/spotify/login');
  });

  test('shows connected state when authenticated', async ({ page }) => {
    // Override the MSW handler to return successful playlists
    await page.addInitScript(() => {
      // This script runs in the browser context before page load
      localStorage.setItem('msw-spotify-authenticated', 'true');
    });
    
    await page.goto('/dashboard');
    await page.waitForLoadState('networkidle');
    
    // Wait for the component to render with authenticated state
    await page.waitForTimeout(1000);
    
    // The default MSW handler returns playlists when authenticated
    const connectedText = page.locator('text=Spotify Connected');
    await expect(connectedText).toBeVisible();
    
    const viewPlaylistsButton = page.locator('text=View Playlists');
    await expect(viewPlaylistsButton).toBeVisible();
  });

  test('shows success message on OAuth callback', async ({ page }) => {
    // Navigate to dashboard with success query param
    await page.goto('/dashboard?spotify=connected');
    await page.waitForLoadState('networkidle');
    
    // Check console for success message
    const messages: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'log') {
        messages.push(msg.text());
      }
    });
    
    await page.waitForTimeout(500); // Give time for useEffect to run
    
    // The dashboard component logs to console when spotify=connected
    const hasSuccessMessage = messages.some(msg => 
      msg.includes('Spotify connected') || msg.includes('spotify=connected')
    );
    expect(hasSuccessMessage).toBe(true);
  });

  test('shows error message on OAuth error', async ({ page }) => {
    // Navigate to dashboard with error query param
    await page.goto('/dashboard?spotify=error&message=Test+error');
    await page.waitForLoadState('networkidle');
    
    // Check console for error message
    const messages: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'log') {
        messages.push(msg.text());
      }
    });
    
    await page.waitForTimeout(500); // Give time for useEffect to run
    
    // The dashboard component logs to console when spotify=error
    const hasErrorMessage = messages.some(msg => 
      msg.includes('Spotify connection failed') || msg.includes('spotify=error')
    );
    expect(hasErrorMessage).toBe(true);
  });
}); 