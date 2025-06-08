import { createLazyFileRoute } from '@tanstack/react-router'

function Dashboard() {
  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        <div className="text-center">
          <h1 className="text-4xl font-extrabold text-gray-900">
            Welcome to Spotube Dashboard
          </h1>
          <p className="mt-4 text-xl text-gray-600">
            Your music streaming application is ready to use!
          </p>
        </div>
      </div>
    </div>
  )
}

export const Route = createLazyFileRoute('/dashboard')({
  component: Dashboard,
}) 