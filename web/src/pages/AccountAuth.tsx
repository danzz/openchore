import React, { useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { api, APIError, setActiveHouseholdId } from '../api';
import { useAuth } from '../AuthContext';
import { BrandMark } from '../components/BrandMark/BrandMark';
import styles from './AccountAuth.module.css';

export const AccountAuth: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const { refresh } = useAuth();

  const isRegisterInitial = new URLSearchParams(location.search).get('tab') === 'register';
  const [tab, setTab] = useState<'login' | 'register'>(isRegisterInitial ? 'register' : 'login');

  // Form states
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [householdName, setHouseholdName] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      if (tab === 'login') {
        const res = await api.account.login({ email, password });
        if (res.households && res.households.length > 0) {
          setActiveHouseholdId(res.households[0].id);
        } else if (res.user?.household_id) {
          setActiveHouseholdId(res.user.household_id);
        }
      } else {
        const res = await api.account.register({
          email,
          password,
          display_name: displayName,
          household_name: householdName,
        });
        if (res.household?.id) {
          setActiveHouseholdId(res.household.id);
        } else if (res.user?.household_id) {
          setActiveHouseholdId(res.user.household_id);
        }
      }

      // Update session in AuthContext
      await refresh();
      navigate('/', { replace: true });
    } catch (err: any) {
      if (err instanceof APIError) {
        setError(err.message || '操作失败，请重试');
      } else {
        setError('网络或服务异常，请重试');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <BrandMark />

        <div className={styles.tabs}>
          <button
            type="button"
            className={`${styles.tabBtn} ${tab === 'login' ? styles.activeTab : ''}`}
            onClick={() => { setTab('login'); setError(null); }}
          >
            账号登录
          </button>
          <button
            type="button"
            className={`${styles.tabBtn} ${tab === 'register' ? styles.activeTab : ''}`}
            onClick={() => { setTab('register'); setError(null); }}
          >
            注册新家庭
          </button>
        </div>

        {error && <div className={styles.errorBanner}>{error}</div>}

        <form className={styles.form} onSubmit={handleSubmit}>
          <div className={styles.field}>
            <label className={styles.label}>邮箱地址 (Email)</label>
            <input
              type="email"
              className={styles.input}
              placeholder="parent@example.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoFocus
            />
          </div>

          <div className={styles.field}>
            <label className={styles.label}>密码 (Password)</label>
            <input
              type="password"
              className={styles.input}
              placeholder="至少 6 位密码"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={6}
            />
          </div>

          {tab === 'register' && (
            <>
              <div className={styles.field}>
                <label className={styles.label}>你的昵称 (家长名)</label>
                <input
                  type="text"
                  className={styles.input}
                  placeholder="例如：爸爸 / 妈妈 / Alice"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  required
                />
              </div>

              <div className={styles.field}>
                <label className={styles.label}>家庭名称 (Household Name)</label>
                <input
                  type="text"
                  className={styles.input}
                  placeholder="例如：快乐一家人 / Smith Family"
                  value={householdName}
                  onChange={(e) => setHouseholdName(e.target.value)}
                  required
                />
              </div>
            </>
          )}

          <button type="submit" className={styles.submitBtn} disabled={isSubmitting}>
            {isSubmitting ? '处理中...' : tab === 'login' ? '登 录' : '创建家庭并开启体验'}
          </button>
        </form>

        <button
          type="button"
          className={styles.backLink}
          onClick={() => navigate('/login')}
        >
          返回家庭成员头像登录
        </button>
      </div>
    </div>
  );
};

export default AccountAuth;
