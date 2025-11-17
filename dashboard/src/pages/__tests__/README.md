# AuditLogs Test Cases

This file contains comprehensive test cases for the AuditLogs component.

## Test Coverage

### 1. Loading State
- ✅ Displays loading message when data is loading

### 2. Error State
- ✅ Displays error message when there is an error

### 3. Empty State
- ✅ Displays empty message when no logs are found

### 4. Data Display
- ✅ Renders audit logs table with correct headers
- ✅ Displays audit log data correctly
- ✅ Displays action badges with correct colors (green for create, yellow for update, red for delete)
- ✅ Formats dates correctly
- ✅ Displays resource ID when available
- ✅ Displays dash for missing optional fields

### 5. Filter Functionality
- ✅ Renders filter inputs (Cluster, Resource, Action)
- ✅ Has Apply Filters and Clear Filters buttons
- ✅ Disables Apply Filters button when no changes are made
- ✅ Enables Apply Filters button when filters are changed
- ✅ Applies filters when Apply Filters button is clicked
- ✅ Clears all filters when Clear Filters button is clicked
- ✅ Resets to page 1 when filters are applied

### 6. Pagination
- ✅ Displays pagination when total pages > 1
- ✅ Does not display pagination when total pages <= 1
- ✅ Disables Previous button on first page
- ✅ Disables Next button on last page
- ✅ Navigates to next page when Next button is clicked
- ✅ Navigates to previous page when Previous button is clicked
- ✅ Displays correct pagination info

### 7. Filter Options
- ✅ Has all cluster options in dropdown
- ✅ Has all resource options in dropdown
- ✅ Has all action options in dropdown

## Running Tests

```bash
# Run all tests
npm test

# Run tests in watch mode
npm test -- --watch

# Run tests with UI
npm run test:ui

# Run tests with coverage
npm run test:coverage
```

## Test Structure

Tests are organized using `describe` blocks for logical grouping:
- Loading State
- Error State
- Empty State
- Data Display
- Filter Functionality
- Pagination
- Filter Options

Each test case uses:
- `renderWithProviders()` helper to render component with QueryClientProvider
- Mocked hooks (`useAuditLogs`, `useClusters`)
- `userEvent` for user interactions
- `waitFor` for async operations

