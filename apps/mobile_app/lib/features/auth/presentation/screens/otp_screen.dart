import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../domain/usecases/send_otp_usecase.dart';
import '../../data/datasources/auth_remote_datasource.dart';
import '../../data/datasources/auth_local_datasource.dart';
import '../../data/repositories/auth_repository_impl.dart';

class OtpScreen extends ConsumerStatefulWidget {
  final String phone;

  const OtpScreen({super.key, required this.phone});

  @override
  ConsumerState<OtpScreen> createState() => _OtpScreenState();
}

class _OtpScreenState extends ConsumerState<OtpScreen> {
  static const _codeLength = 6;
  static const _resendSeconds = 60;

  final List<TextEditingController> _controllers =
      List.generate(_codeLength, (_) => TextEditingController());
  final List<FocusNode> _focusNodes =
      List.generate(_codeLength, (_) => FocusNode());

  int _secondsLeft = _resendSeconds;
  Timer? _timer;
  bool _isVerifying = false;

  @override
  void initState() {
    super.initState();
    _startTimer();
  }

  @override
  void dispose() {
    _timer?.cancel();
    for (final c in _controllers) {
      c.dispose();
    }
    for (final f in _focusNodes) {
      f.dispose();
    }
    super.dispose();
  }

  // -------------------------------------------------------------------------
  // Timer
  // -------------------------------------------------------------------------

  void _startTimer() {
    _timer?.cancel();
    setState(() => _secondsLeft = _resendSeconds);
    _timer = Timer.periodic(const Duration(seconds: 1), (t) {
      if (_secondsLeft == 0) {
        t.cancel();
      } else {
        setState(() => _secondsLeft--);
      }
    });
  }

  // -------------------------------------------------------------------------
  // OTP entry
  // -------------------------------------------------------------------------

  String get _currentCode =>
      _controllers.map((c) => c.text).join();

  void _onDigitChanged(int index, String value) {
    if (value.length > 1) {
      // Handle paste: distribute digits across fields.
      final digits = value.replaceAll(RegExp(r'\D'), '');
      for (int i = 0; i < _codeLength && i < digits.length; i++) {
        _controllers[i].text = digits[i];
      }
      final nextEmpty =
          _controllers.indexWhere((c) => c.text.isEmpty);
      final focusTarget =
          nextEmpty == -1 ? _codeLength - 1 : nextEmpty;
      _focusNodes[focusTarget].requestFocus();
      if (_currentCode.length == _codeLength) _submit();
      return;
    }

    if (value.isNotEmpty) {
      _controllers[index].text = value;
      if (index < _codeLength - 1) {
        _focusNodes[index + 1].requestFocus();
      } else {
        _focusNodes[index].unfocus();
        _submit();
      }
    }
  }

  void _onKeyEvent(int index, KeyEvent event) {
    if (event is KeyDownEvent &&
        event.logicalKey == LogicalKeyboardKey.backspace &&
        _controllers[index].text.isEmpty &&
        index > 0) {
      _focusNodes[index - 1].requestFocus();
    }
  }

  Future<void> _submit() async {
    final code = _currentCode;
    if (code.length < _codeLength || _isVerifying) return;
    setState(() => _isVerifying = true);

    final repo = AuthRepositoryImpl(
      remote: AuthRemoteDataSourceImpl(),
      local: const AuthLocalDataSourceImpl(),
    );
    final result = await repo.verifyOTP(phone: widget.phone, code: code);

    if (!mounted) return;
    setState(() => _isVerifying = false);

    result.fold(
      (failure) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(failure.message),
            backgroundColor: const Color(0xFFFF0050),
            behavior: SnackBarBehavior.floating,
          ),
        );
        // Clear fields on failure.
        for (final c in _controllers) {
          c.clear();
        }
        _focusNodes[0].requestFocus();
      },
      (_) => context.go('/home'),
    );
  }

  Future<void> _resend() async {
    if (_secondsLeft > 0) return;
    final repo = AuthRepositoryImpl(
      remote: AuthRemoteDataSourceImpl(),
      local: const AuthLocalDataSourceImpl(),
    );
    final useCase = SendOtpUseCase(repo);
    final result = await useCase(SendOtpParams(phone: widget.phone));
    if (!mounted) return;
    result.fold(
      (failure) => ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(failure.message),
          backgroundColor: const Color(0xFFFF0050),
          behavior: SnackBarBehavior.floating,
        ),
      ),
      (_) {
        _startTimer();
        for (final c in _controllers) {
          c.clear();
        }
        _focusNodes[0].requestFocus();
      },
    );
  }

  // -------------------------------------------------------------------------
  // Build
  // -------------------------------------------------------------------------

  @override
  Widget build(BuildContext context) {
    final canResend = _secondsLeft == 0;

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.arrow_back_ios, color: Colors.white, size: 20),
          onPressed: () => context.pop(),
        ),
      ),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              const SizedBox(height: 32),
              const Text(
                'Enter verification code',
                style: TextStyle(
                  color: Colors.white,
                  fontSize: 24,
                  fontWeight: FontWeight.w700,
                ),
              ),
              const SizedBox(height: 12),
              RichText(
                textAlign: TextAlign.center,
                text: TextSpan(
                  style: TextStyle(
                    color: Colors.grey[400],
                    fontSize: 14,
                    height: 1.5,
                  ),
                  children: [
                    const TextSpan(text: 'We sent a 6-digit code to\n'),
                    TextSpan(
                      text: widget.phone,
                      style: const TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ],
                ),
              ),

              const SizedBox(height: 48),

              // OTP input row.
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                children: List.generate(_codeLength, (i) {
                  return SizedBox(
                    width: 46,
                    height: 56,
                    child: KeyboardListener(
                      focusNode: FocusNode(),
                      onKeyEvent: (e) => _onKeyEvent(i, e),
                      child: TextFormField(
                        controller: _controllers[i],
                        focusNode: _focusNodes[i],
                        keyboardType: TextInputType.number,
                        textAlign: TextAlign.center,
                        maxLength: 1,
                        inputFormatters: [
                          FilteringTextInputFormatter.digitsOnly,
                          LengthLimitingTextInputFormatter(1),
                        ],
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 22,
                          fontWeight: FontWeight.w700,
                        ),
                        decoration: InputDecoration(
                          counterText: '',
                          filled: true,
                          fillColor: const Color(0xFF1A1A1A),
                          border: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(8),
                            borderSide: BorderSide.none,
                          ),
                          enabledBorder: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(8),
                            borderSide: BorderSide(
                              color: Colors.grey[800]!,
                              width: 1,
                            ),
                          ),
                          focusedBorder: OutlineInputBorder(
                            borderRadius: BorderRadius.circular(8),
                            borderSide: const BorderSide(
                              color: Color(0xFFFF0050),
                              width: 2,
                            ),
                          ),
                        ),
                        onChanged: (v) => _onDigitChanged(i, v),
                      ),
                    ),
                  );
                }),
              ),

              const SizedBox(height: 40),

              // Loading indicator or resend.
              if (_isVerifying)
                const CircularProgressIndicator(color: Color(0xFFFF0050))
              else ...[
                GestureDetector(
                  onTap: canResend ? _resend : null,
                  child: RichText(
                    text: TextSpan(
                      style: const TextStyle(fontSize: 14),
                      children: [
                        TextSpan(
                          text: "Didn't receive the code? ",
                          style: TextStyle(color: Colors.grey[500]),
                        ),
                        TextSpan(
                          text: canResend
                              ? 'Resend'
                              : 'Resend in ${_secondsLeft}s',
                          style: TextStyle(
                            color: canResend
                                ? const Color(0xFFFF0050)
                                : Colors.grey[600],
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
