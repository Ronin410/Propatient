import { describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ConfirmDialog } from './ConfirmDialog';

describe('ConfirmDialog', () => {
  it('no renderiza nada cuando isOpen es false', () => {
    const { container } = render(
      <ConfirmDialog
        isOpen={false}
        title="Título"
        message="Mensaje"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('muestra título, mensaje y textos de botones por defecto', () => {
    render(
      <ConfirmDialog
        isOpen={true}
        title="Eliminar paciente"
        message="¿Seguro?"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />
    );
    expect(screen.getByText('Eliminar paciente')).toBeInTheDocument();
    expect(screen.getByText('¿Seguro?')).toBeInTheDocument();
    expect(screen.getByText('Confirmar')).toBeInTheDocument();
    expect(screen.getByText('Cancelar')).toBeInTheDocument();
  });

  it('llama a onConfirm al hacer clic en el botón de confirmar', async () => {
    const onConfirm = vi.fn();
    const user = userEvent.setup();
    render(
      <ConfirmDialog
        isOpen={true}
        title="t"
        message="m"
        confirmText="Eliminar"
        onConfirm={onConfirm}
        onCancel={vi.fn()}
      />
    );
    await user.click(screen.getByText('Eliminar'));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('llama a onCancel al hacer clic en cancelar o en el overlay', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(
      <ConfirmDialog
        isOpen={true}
        title="t"
        message="m"
        onConfirm={vi.fn()}
        onCancel={onCancel}
      />
    );
    await user.click(screen.getByText('Cancelar'));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it('no cierra al hacer clic dentro del contenido (stopPropagation)', async () => {
    const onCancel = vi.fn();
    const user = userEvent.setup();
    render(
      <ConfirmDialog
        isOpen={true}
        title="Título del diálogo"
        message="m"
        onConfirm={vi.fn()}
        onCancel={onCancel}
      />
    );
    await user.click(screen.getByText('Título del diálogo'));
    expect(onCancel).not.toHaveBeenCalled();
  });
});
