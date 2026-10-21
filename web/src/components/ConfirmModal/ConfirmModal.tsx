import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AlertTriangle, Trash2 } from 'lucide-react';
import Modal from '../Modal/Modal';
import styles from './ConfirmModal.module.css';

export interface ConfirmModalProps {
  isOpen: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  isDanger?: boolean;
  onConfirm: () => Promise<void> | void;
  onCancel: () => void;
}

export const ConfirmModal: React.FC<ConfirmModalProps> = ({
  isOpen,
  title,
  message,
  confirmLabel,
  cancelLabel,
  isDanger = true,
  onConfirm,
  onCancel,
}) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  if (!isOpen) return null;

  const handleConfirm = async () => {
    setLoading(true);
    setError('');
    try {
      await onConfirm();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err));
      setLoading(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={onCancel} title={title} maxWidth="460px">
      <div className={styles.content}>
        <div className={styles.body}>
          <div className={styles.iconWell}>
            {isDanger ? <Trash2 aria-hidden /> : <AlertTriangle aria-hidden />}
          </div>
          <p className={styles.message}>{message}</p>
        </div>

        {error && <div className={styles.error} role="alert">{error}</div>}

        <div className={styles.actions}>
          <button
            type="button"
            className={styles.btnCancel}
            onClick={onCancel}
            disabled={loading}
          >
            {cancelLabel || t('common.cancel', { defaultValue: '取消' })}
          </button>
          <button
            type="button"
            className={styles.btnConfirmDanger}
            onClick={handleConfirm}
            disabled={loading}
          >
            {isDanger && <Trash2 aria-hidden style={{ width: 16, height: 16 }} />}
            {loading
              ? t('common.loading', { defaultValue: '正在处理...' })
              : (confirmLabel || t('common.confirm', { defaultValue: '确认删除' }))}
          </button>
        </div>
      </div>
    </Modal>
  );
};

export default ConfirmModal;
