import React from 'react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import AuditLogs from '../AuditLogs'
import { useAuditLogs } from '../../hooks/useAuditLogs'
import { useClusters } from '../../hooks/useClusters'
import type { AuditLog } from '../../services/api'

// Mock the hooks
vi.mock('../../hooks/useAuditLogs')
vi.mock('../../hooks/useClusters')

const mockUseAuditLogs = vi.mocked(useAuditLogs)
const mockUseClusters = vi.mocked(useClusters)

// Helper function to create a test query client
const createTestQueryClient = () => {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })
}

// Helper function to render component with providers
const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = createTestQueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      {ui}
    </QueryClientProvider>
  )
}

// Mock data
const mockClusters = [
  { id: 'cluster1', name: 'Cluster 1' },
  { id: 'cluster2', name: 'Cluster 2' },
]

const mockAuditLogs: AuditLog[] = [
  {
    id: 1,
    clusterId: 'cluster1',
    action: 'create',
    resource: 'serviceaccount',
    resourceId: 'sa-123',
    details: 'Created service account',
    user: 'admin',
    ip: '192.168.1.1',
    createdAt: '2024-01-15T10:30:00Z',
  },
  {
    id: 2,
    clusterId: 'cluster2',
    action: 'update',
    resource: 'rolebinding',
    resourceId: 'rb-456',
    details: 'Updated role binding',
    user: 'user1',
    ip: '192.168.1.2',
    createdAt: '2024-01-15T11:45:00Z',
  },
  {
    id: 3,
    clusterId: 'cluster1',
    action: 'delete',
    resource: 'clusterrole',
    user: 'admin',
    ip: '192.168.1.3',
    createdAt: '2024-01-15T12:00:00Z',
  },
]

describe('AuditLogs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockUseClusters.mockReturnValue({
      data: mockClusters,
      isLoading: false,
      error: null,
    } as any)
  })

  describe('Loading State', () => {
    it('should display loading message when data is loading', () => {
      mockUseAuditLogs.mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
      } as any)

      renderWithProviders(<AuditLogs />)
      expect(screen.getByText('Loading...')).toBeInTheDocument()
    })
  })

  describe('Error State', () => {
    it('should display error message when there is an error', () => {
      mockUseAuditLogs.mockReturnValue({
        data: undefined,
        isLoading: false,
        error: new Error('Failed to load audit logs'),
      } as any)

      renderWithProviders(<AuditLogs />)
      expect(screen.getByText(/Error loading audit logs/)).toBeInTheDocument()
    })
  })

  describe('Empty State', () => {
    it('should display empty message when no logs are found', () => {
      mockUseAuditLogs.mockReturnValue({
        data: { logs: [], total: 0, page: 1, pageSize: 20 },
        isLoading: false,
        error: null,
      } as any)

      renderWithProviders(<AuditLogs />)
      expect(screen.getByText('No audit logs found')).toBeInTheDocument()
    })
  })

  describe('Data Display', () => {
    beforeEach(() => {
      mockUseAuditLogs.mockReturnValue({
        data: { logs: mockAuditLogs, total: 3, page: 1, pageSize: 20 },
        isLoading: false,
        error: null,
      } as any)
    })

    it('should render audit logs table with correct headers', () => {
      renderWithProviders(<AuditLogs />)
      
      // Use getAllByText for headers that might appear multiple times
      expect(screen.getAllByText('Timestamp')[0]).toBeInTheDocument()
      expect(screen.getAllByText('Action')[0]).toBeInTheDocument()
      expect(screen.getAllByText('Resource')[0]).toBeInTheDocument()
      expect(screen.getAllByText('Cluster')[0]).toBeInTheDocument()
      expect(screen.getAllByText('User')[0]).toBeInTheDocument()
      expect(screen.getByText('IP Address')).toBeInTheDocument()
      expect(screen.getByText('Details')).toBeInTheDocument()
    })

    it('should display audit log data correctly', () => {
      renderWithProviders(<AuditLogs />)
      
      // Check first log
      expect(screen.getByText('serviceaccount')).toBeInTheDocument()
      expect(screen.getByText('sa-123')).toBeInTheDocument()
      expect(screen.getByText('admin')).toBeInTheDocument()
      expect(screen.getByText('192.168.1.1')).toBeInTheDocument()
      expect(screen.getByText('Created service account')).toBeInTheDocument()
    })

    it('should display action badges with correct colors', () => {
      renderWithProviders(<AuditLogs />)
      
      const createBadge = screen.getByText('create')
      const updateBadge = screen.getByText('update')
      const deleteBadge = screen.getByText('delete')
      
      expect(createBadge).toHaveClass('bg-green-100', 'text-green-800')
      expect(updateBadge).toHaveClass('bg-yellow-100', 'text-yellow-800')
      expect(deleteBadge).toHaveClass('bg-red-100', 'text-red-800')
    })

    it('should format dates correctly', () => {
      renderWithProviders(<AuditLogs />)
      
      // Check if formatted date is displayed (format: "Jan 15, 2024")
      const dateElements = screen.getAllByText(/Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec/)
      expect(dateElements.length).toBeGreaterThan(0)
    })

    it('should display resource ID when available', () => {
      renderWithProviders(<AuditLogs />)
      
      expect(screen.getByText('sa-123')).toBeInTheDocument()
      expect(screen.getByText('rb-456')).toBeInTheDocument()
    })

    it('should display dash for missing optional fields', () => {
      renderWithProviders(<AuditLogs />)
      
      // Third log doesn't have resourceId or details
      const dashes = screen.getAllByText('-')
      expect(dashes.length).toBeGreaterThan(0)
    })
  })

  describe('Filter Functionality', () => {
    beforeEach(() => {
      mockUseAuditLogs.mockReturnValue({
        data: { logs: mockAuditLogs, total: 3, page: 1, pageSize: 20 },
        isLoading: false,
        error: null,
      } as any)
    })

    it('should render filter inputs', () => {
      renderWithProviders(<AuditLogs />)
      
      expect(screen.getByLabelText('Cluster')).toBeInTheDocument()
      expect(screen.getByLabelText('Resource')).toBeInTheDocument()
      expect(screen.getByLabelText('Action')).toBeInTheDocument()
    })

    it('should have Apply Filters and Clear Filters buttons', () => {
      renderWithProviders(<AuditLogs />)
      
      expect(screen.getByText('Apply Filters')).toBeInTheDocument()
      expect(screen.getByText('Clear Filters')).toBeInTheDocument()
    })

    it('should disable Apply Filters button when no changes are made', () => {
      renderWithProviders(<AuditLogs />)
      
      const applyButton = screen.getByText('Apply Filters')
      expect(applyButton).toBeDisabled()
    })

    it('should enable Apply Filters button when filters are changed', async () => {
      const user = userEvent.setup()
      renderWithProviders(<AuditLogs />)
      
      const clusterSelect = screen.getByLabelText('Cluster')
      await user.selectOptions(clusterSelect, 'cluster1')
      
      const applyButton = screen.getByText('Apply Filters')
      expect(applyButton).not.toBeDisabled()
    })

    it('should apply filters when Apply Filters button is clicked', async () => {
      const user = userEvent.setup()
      let callCount = 0
      
      mockUseAuditLogs.mockImplementation((params) => {
        callCount++
        return {
          data: { logs: mockAuditLogs, total: 3, page: 1, pageSize: 20 },
          isLoading: false,
          error: null,
        } as any
      })
      
      renderWithProviders(<AuditLogs />)
      
      const clusterSelect = screen.getByLabelText('Cluster')
      await user.selectOptions(clusterSelect, 'cluster1')
      
      const applyButton = screen.getByText('Apply Filters')
      await user.click(applyButton)
      
      // Wait for the hook to be called with new params
      await waitFor(() => {
        const calls = mockUseAuditLogs.mock.calls
        const lastCall = calls[calls.length - 1]
        expect(lastCall[0]?.cluster).toBe('cluster1')
      })
    })

    it('should clear all filters when Clear Filters button is clicked', async () => {
      const user = userEvent.setup()
      renderWithProviders(<AuditLogs />)
      
      // Set some filters
      const clusterSelect = screen.getByLabelText('Cluster')
      const resourceSelect = screen.getByLabelText('Resource')
      
      await user.selectOptions(clusterSelect, 'cluster1')
      await user.selectOptions(resourceSelect, 'serviceaccount')
      
      // Clear filters
      const clearButton = screen.getByText('Clear Filters')
      await user.click(clearButton)
      
      // Check that filters are cleared
      expect(clusterSelect).toHaveValue('')
      expect(resourceSelect).toHaveValue('')
    })

    it('should reset to page 1 when filters are applied', async () => {
      const user = userEvent.setup()
      let callCount = 0
      
      mockUseAuditLogs.mockImplementation((params) => {
        callCount++
        return {
          data: { logs: mockAuditLogs, total: 3, page: params?.page || 1, pageSize: 20 },
          isLoading: false,
          error: null,
        } as any
      })
      
      renderWithProviders(<AuditLogs />)
      
      const clusterSelect = screen.getByLabelText('Cluster')
      await user.selectOptions(clusterSelect, 'cluster1')
      
      const applyButton = screen.getByText('Apply Filters')
      await user.click(applyButton)
      
      // Should reset to page 1
      await waitFor(() => {
        const calls = mockUseAuditLogs.mock.calls
        const lastCall = calls[calls.length - 1]
        expect(lastCall[0]?.page).toBe(1)
      })
    })
  })

  describe('Pagination', () => {
    beforeEach(() => {
      mockUseAuditLogs.mockReturnValue({
        data: { logs: mockAuditLogs, total: 25, page: 1, pageSize: 20 },
        isLoading: false,
        error: null,
      } as any)
    })

    it('should display pagination when total pages > 1', () => {
      renderWithProviders(<AuditLogs />)
      
      expect(screen.getByText(/Showing/)).toBeInTheDocument()
      expect(screen.getByText('Previous')).toBeInTheDocument()
      expect(screen.getByText('Next')).toBeInTheDocument()
    })

    it('should not display pagination when total pages <= 1', () => {
      mockUseAuditLogs.mockReturnValue({
        data: { logs: mockAuditLogs, total: 3, page: 1, pageSize: 20 },
        isLoading: false,
        error: null,
      } as any)
      
      renderWithProviders(<AuditLogs />)
      
      expect(screen.queryByText('Previous')).not.toBeInTheDocument()
      expect(screen.queryByText('Next')).not.toBeInTheDocument()
    })

    it('should disable Previous button on first page', () => {
      renderWithProviders(<AuditLogs />)
      
      const previousButton = screen.getByText('Previous')
      expect(previousButton).toBeDisabled()
    })

    it('should disable Next button on last page', async () => {
      mockUseAuditLogs.mockReturnValue({
        data: { logs: mockAuditLogs, total: 25, page: 2, pageSize: 20 },
        isLoading: false,
        error: null,
      } as any)
      
      const { rerender } = renderWithProviders(<AuditLogs />)
      
      rerender(
        <QueryClientProvider client={createTestQueryClient()}>
          <AuditLogs />
        </QueryClientProvider>
      )
      
      await waitFor(() => {
        const nextButton = screen.getByText('Next')
        expect(nextButton).toBeDisabled()
      })
    })

    it('should navigate to next page when Next button is clicked', async () => {
      const user = userEvent.setup()
      let callCount = 0
      
      mockUseAuditLogs.mockImplementation((params) => {
        callCount++
        if (callCount === 1) {
          return {
            data: { logs: mockAuditLogs, total: 25, page: 1, pageSize: 20 },
            isLoading: false,
            error: null,
          } as any
        }
        return {
          data: { logs: mockAuditLogs, total: 25, page: params?.page || 2, pageSize: 20 },
          isLoading: false,
          error: null,
        } as any
      })
      
      renderWithProviders(<AuditLogs />)
      
      const nextButton = screen.getByText('Next')
      await user.click(nextButton)
      
      // Should call hook with page 2
      await waitFor(() => {
        const calls = mockUseAuditLogs.mock.calls
        const lastCall = calls[calls.length - 1]
        expect(lastCall[0]?.page).toBe(2)
      })
    })

    it('should navigate to previous page when Previous button is clicked', async () => {
      const user = userEvent.setup()
      let callCount = 0
      
      mockUseAuditLogs.mockImplementation((params) => {
        callCount++
        if (callCount === 1) {
          return {
            data: { logs: mockAuditLogs, total: 25, page: 2, pageSize: 20 },
            isLoading: false,
            error: null,
          } as any
        }
        return {
          data: { logs: mockAuditLogs, total: 25, page: params?.page || 1, pageSize: 20 },
          isLoading: false,
          error: null,
        } as any
      })
      
      renderWithProviders(<AuditLogs />)
      
      await waitFor(() => {
        const previousButton = screen.getByText('Previous')
        expect(previousButton).not.toBeDisabled()
      })
      
      const previousButton = screen.getByText('Previous')
      await user.click(previousButton)
      
      // Should call hook with page 1
      await waitFor(() => {
        const calls = mockUseAuditLogs.mock.calls
        const lastCall = calls[calls.length - 1]
        expect(lastCall[0]?.page).toBe(1)
      })
    })

    it('should display correct pagination info', () => {
      renderWithProviders(<AuditLogs />)
      
      expect(screen.getByText(/Showing 1 to 20 of 25 results/)).toBeInTheDocument()
    })
  })

  describe('Filter Options', () => {
    beforeEach(() => {
      mockUseAuditLogs.mockReturnValue({
        data: { logs: mockAuditLogs, total: 3, page: 1, pageSize: 20 },
        isLoading: false,
        error: null,
      } as any)
    })

    it('should have all cluster options in dropdown', () => {
      renderWithProviders(<AuditLogs />)
      
      // Check that cluster options are present in the document
      expect(screen.getByText('All Clusters')).toBeInTheDocument()
      expect(screen.getByText('Cluster 1')).toBeInTheDocument()
      expect(screen.getByText('Cluster 2')).toBeInTheDocument()
    })

    it('should have all resource options in dropdown', () => {
      renderWithProviders(<AuditLogs />)
      
      // Check that resource options are present in the document
      expect(screen.getByText('All Resources')).toBeInTheDocument()
      expect(screen.getByText('ServiceAccount')).toBeInTheDocument()
      expect(screen.getByText('RoleBinding')).toBeInTheDocument()
      expect(screen.getByText('ClusterRoleBinding')).toBeInTheDocument()
      expect(screen.getByText('Role')).toBeInTheDocument()
      expect(screen.getByText('ClusterRole')).toBeInTheDocument()
    })

    it('should have all action options in dropdown', () => {
      renderWithProviders(<AuditLogs />)
      
      // Check that action options are present in the document
      expect(screen.getByText('All Actions')).toBeInTheDocument()
      expect(screen.getByText('Create')).toBeInTheDocument()
      expect(screen.getByText('Update')).toBeInTheDocument()
      expect(screen.getByText('Delete')).toBeInTheDocument()
    })
  })
})

