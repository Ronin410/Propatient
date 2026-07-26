import { describe, expect, it, vi } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useFetchData } from './useFetchData';

describe('useFetchData', () => {
  it('empieza en loading y expone los datos cuando el fetcher resuelve', async () => {
    const fetcher = vi.fn().mockResolvedValue({ hello: 'world' });
    const { result } = renderHook(() => useFetchData(fetcher));

    expect(result.current.loading).toBe(true);
    expect(result.current.data).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.data).toEqual({ hello: 'world' });
    expect(result.current.error).toBeNull();
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it('expone un error legible cuando el fetcher rechaza', async () => {
    const fetcher = vi.fn().mockRejectedValue(new Error('boom'));
    const { result } = renderHook(() => useFetchData(fetcher));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe('No se pudo cargar la información.');
    expect(result.current.data).toBeNull();
  });

  it('refetch() vuelve a llamar al fetcher', async () => {
    const fetcher = vi.fn().mockResolvedValue('v1');
    const { result } = renderHook(() => useFetchData(fetcher));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(fetcher).toHaveBeenCalledTimes(1);

    act(() => {
      result.current.refetch();
    });

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
  });

  it('vuelve a llamar al fetcher cuando cambian las dependencias', async () => {
    const fetcher = vi.fn().mockResolvedValue('data');
    const { rerender } = renderHook(({ id }) => useFetchData(fetcher, [id]), {
      initialProps: { id: 1 },
    });

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(1));

    rerender({ id: 2 });

    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
  });
});
