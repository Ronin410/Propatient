import React from 'react';
import './ConfirmDialog.scss';

interface ConfirmDialogProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  variant?: 'default' | 'danger';
  onConfirm: () => void;
  onCancel: () => void;
}

export const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  isOpen,
  title,
  message,
  confirmText = 'Confirmar',
  cancelText = 'Cancelar',
  variant = 'default',
  onConfirm,
  onCancel,
}) => {
  if (!isOpen) return null;

  return (
    <div className="confirm-dialog-overlay" onClick={onCancel}>
      <div className="confirm-dialog-content" onClick={(e) => e.stopPropagation()}>
        <span className={`material-icons-outlined confirm-dialog-icon icon-${variant}`}>
          {variant === 'danger' ? 'warning' : 'help_outline'}
        </span>
        <h3>{title}</h3>
        <p>{message}</p>
        <div className="confirm-dialog-actions">
          <button className="btn-cancel" onClick={onCancel}>
            {cancelText}
          </button>
          <button className={`btn-confirm btn-${variant}`} onClick={onConfirm}>
            {confirmText}
          </button>
        </div>
      </div>
    </div>
  );
};
