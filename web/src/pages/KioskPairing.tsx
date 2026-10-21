import React, { useEffect, useState, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { api, setDeviceToken } from '../api';
import { BrandMark } from '../components/BrandMark/BrandMark';
import styles from './KioskPairing.module.css';

export const KioskPairing: React.FC = () => {
  const navigate = useNavigate();
  const [pairCode, setPairCode] = useState<string>('');
  const [timeLeft, setTimeLeft] = useState<number>(600);
  const [isExpired, setIsExpired] = useState<boolean>(false);
  const [isPaired, setIsPaired] = useState<boolean>(false);
  const [deviceName, setDeviceName] = useState<string>('');
  const pollTimerRef = useRef<number | null>(null);

  const initPairing = async () => {
    try {
      setIsExpired(false);
      setIsPaired(false);
      const res = await api.kiosk.init();
      setPairCode(res.pair_code);
      setTimeLeft(res.expires_in || 600);
    } catch (err) {
      console.error('Failed to init kiosk pairing:', err);
    }
  };

  useEffect(() => {
    initPairing();
    return () => {
      if (pollTimerRef.current) clearInterval(pollTimerRef.current);
    };
  }, []);

  // Countdown timer
  useEffect(() => {
    if (!pairCode || isPaired || isExpired) return;
    const timer = setInterval(() => {
      setTimeLeft((prev) => {
        if (prev <= 1) {
          clearInterval(timer);
          setIsExpired(true);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
    return () => clearInterval(timer);
  }, [pairCode, isPaired, isExpired]);

  // Polling for claim status
  useEffect(() => {
    if (!pairCode || isPaired || isExpired) return;

    const poll = async () => {
      try {
        const res = await api.kiosk.poll(pairCode);
        if (res.paired && res.device_token) {
          setIsPaired(true);
          setDeviceName(res.device_name || '壁挂平板');
          setDeviceToken(res.device_token);

          // Clear timer
          if (pollTimerRef.current) clearInterval(pollTimerRef.current);

          // Redirect to login (profile selection) after 1.5s
          setTimeout(() => {
            navigate('/login', { replace: true });
          }, 1500);
        } else if (res.expired) {
          setIsExpired(true);
        }
      } catch (err) {
        console.error('Poll error:', err);
      }
    };

    pollTimerRef.current = window.setInterval(poll, 2000);
    return () => {
      if (pollTimerRef.current) clearInterval(pollTimerRef.current);
    };
  }, [pairCode, isPaired, isExpired, navigate]);

  const formatTime = (secs: number) => {
    const m = Math.floor(secs / 60);
    const s = secs % 60;
    return `${m}:${s < 10 ? '0' : ''}${s}`;
  };

  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <BrandMark />

        {isPaired ? (
          <div className={styles.successCard}>
            <div className={styles.successIcon}>✓</div>
            <h2 className={styles.title}>设备配对成功！</h2>
            <p className={styles.subtitle}>已成功绑定至家庭设备：{deviceName}，正在进入...</p>
          </div>
        ) : (
          <>
            <h1 className={styles.title}>设置壁挂平板 (Wall Display)</h1>
            <p className={styles.subtitle}>
              在家长手机上登录 OpenChore，进入<strong>【管理 → 壁挂设备】</strong>输入下方 6 位配对码进行绑定：
            </p>

            <div className={styles.codeBox}>
              {pairCode ? (
                pairCode.split('').map((char, idx) => (
                  <div key={idx} className={styles.digit}>
                    {char}
                  </div>
                ))
              ) : (
                <div className={styles.subtitle}>正在生成配对码...</div>
              )}
            </div>

            {isExpired ? (
              <div className={styles.timer} style={{ color: '#ef4444' }}>
                配对码已失效，请重新生成
              </div>
            ) : (
              <div className={styles.timer}>
                <span className={styles.pulseDot} />
                等待家长手机确认 · 有效期 {formatTime(timeLeft)}
              </div>
            )}

            <div className={styles.actions}>
              {isExpired ? (
                <button type="button" className={styles.refreshBtn} onClick={initPairing}>
                  重新生成配对码
                </button>
              ) : null}
              <button
                type="button"
                className={styles.cancelBtn}
                onClick={() => navigate('/login', { replace: true })}
              >
                跳过配对，直接访问
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
};

export default KioskPairing;
