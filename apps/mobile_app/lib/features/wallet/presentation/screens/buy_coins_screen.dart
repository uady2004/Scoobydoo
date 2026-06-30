import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../domain/entities/wallet_entity.dart';
import '../providers/wallet_provider.dart';

class BuyCoinsScreen extends ConsumerWidget {
  const BuyCoinsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final packagesAsync = ref.watch(coinPackagesProvider);

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: const Text(
          'Get Coins',
          style: TextStyle(
            color: Colors.white,
            fontWeight: FontWeight.bold,
            fontSize: 18,
          ),
        ),
        centerTitle: true,
        elevation: 0,
      ),
      body: packagesAsync.when(
        data: (packages) => packages.isEmpty
            ? const _EmptyPackages()
            : _PackageGrid(packages: packages),
        loading: () => const Center(
          child: CircularProgressIndicator(color: Color(0xFFFF2D55)),
        ),
        error: (e, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(Icons.error_outline,
                    color: Color(0xFFFF2D55), size: 48),
                const SizedBox(height: 16),
                Text(
                  e.toString(),
                  style: const TextStyle(color: Color(0xFF888888)),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 20),
                TextButton(
                  onPressed: () => ref.invalidate(coinPackagesProvider),
                  child: const Text('Try again',
                      style: TextStyle(color: Color(0xFFFF2D55))),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Package grid
// ---------------------------------------------------------------------------

class _PackageGrid extends StatelessWidget {
  const _PackageGrid({required this.packages});

  final List<CoinPackageEntity> packages;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Padding(
          padding: EdgeInsets.fromLTRB(16, 12, 16, 4),
          child: Text(
            'Choose a package',
            style: TextStyle(
              color: Color(0xFF888888),
              fontSize: 13,
              fontWeight: FontWeight.w500,
            ),
          ),
        ),
        Expanded(
          child: GridView.count(
            crossAxisCount: 2,
            padding: const EdgeInsets.all(12),
            crossAxisSpacing: 12,
            mainAxisSpacing: 12,
            childAspectRatio: 0.85,
            children: packages
                .map((pkg) => _CoinPackageCard(package: pkg))
                .toList(),
          ),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Package card
// ---------------------------------------------------------------------------

class _CoinPackageCard extends ConsumerWidget {
  const _CoinPackageCard({required this.package});

  final CoinPackageEntity package;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return GestureDetector(
      onTap: () => _showPurchaseSheet(context, ref),
      child: Stack(
        children: [
          // Main card
          Container(
            decoration: BoxDecoration(
              gradient: package.isBestValue
                  ? const LinearGradient(
                      colors: [Color(0xFFFF2D55), Color(0xFFFF6B8A)],
                      begin: Alignment.topLeft,
                      end: Alignment.bottomRight,
                    )
                  : null,
              color: package.isBestValue ? null : const Color(0xFF1A1A1A),
              borderRadius: BorderRadius.circular(14),
              border: Border.all(
                color: package.isBestValue
                    ? Colors.transparent
                    : const Color(0xFF2A2A2A),
                width: 1.5,
              ),
              boxShadow: package.isBestValue
                  ? [
                      BoxShadow(
                        color: const Color(0xFFFF2D55).withValues(alpha: 0.3),
                        blurRadius: 12,
                        offset: const Offset(0, 4),
                      ),
                    ]
                  : null,
            ),
            padding: const EdgeInsets.all(16),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                // Coin icon
                Container(
                  width: 52,
                  height: 52,
                  decoration: const BoxDecoration(
                    color: Color(0xFFFFD700),
                    shape: BoxShape.circle,
                  ),
                  child: const Icon(
                    Icons.monetization_on,
                    color: Colors.white,
                    size: 32,
                  ),
                ),
                const SizedBox(height: 12),
                // Coin amount
                Text(
                  NumberFormat.compact().format(package.coins),
                  style: TextStyle(
                    color: package.isBestValue
                        ? Colors.white
                        : const Color(0xFFFFD700),
                    fontSize: 26,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  'Coins',
                  style: TextStyle(
                    color: package.isBestValue
                        ? Colors.white.withValues(alpha: 0.85)
                        : const Color(0xFF888888),
                    fontSize: 13,
                  ),
                ),
                // Bonus badge
                if (package.bonusCoins > 0) ...[
                  const SizedBox(height: 8),
                  Container(
                    padding:
                        const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: package.isBestValue
                          ? Colors.white.withValues(alpha: 0.2)
                          : const Color(0xFF4CAF50).withValues(alpha: 0.15),
                      borderRadius: BorderRadius.circular(20),
                    ),
                    child: Text(
                      '+${NumberFormat.compact().format(package.bonusCoins)} bonus',
                      style: TextStyle(
                        color: package.isBestValue
                            ? Colors.white
                            : const Color(0xFF4CAF50),
                        fontSize: 11,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ),
                ],
                const SizedBox(height: 10),
                // Price
                Text(
                  '\$${package.price.toStringAsFixed(2)}',
                  style: TextStyle(
                    color: package.isBestValue
                        ? Colors.white
                        : const Color(0xFF888888),
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
          ),
          // "Best Value" diagonal ribbon
          if (package.isBestValue)
            Positioned(
              top: 0,
              right: 0,
              child: ClipRRect(
                borderRadius: const BorderRadius.only(
                  topRight: Radius.circular(14),
                ),
                child: CustomPaint(
                  size: const Size(64, 64),
                  painter: _RibbonPainter(),
                  child: const SizedBox(
                    width: 64,
                    height: 64,
                    child: Align(
                      alignment: Alignment(0.55, -0.55),
                      child: RotatedBox(
                        quarterTurns: 1,
                        child: Text(
                          'BEST',
                          style: TextStyle(
                            color: Colors.white,
                            fontSize: 9,
                            fontWeight: FontWeight.bold,
                            letterSpacing: 0.5,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }

  void _showPurchaseSheet(BuildContext context, WidgetRef ref) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (_) => _PurchaseConfirmSheet(
        package: package,
        ref: ref,
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Diagonal ribbon painter
// ---------------------------------------------------------------------------

class _RibbonPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()..color = Colors.white.withValues(alpha: 0.25);
    final path = Path()
      ..moveTo(0, 0)
      ..lineTo(size.width, 0)
      ..lineTo(size.width, size.height)
      ..close();
    canvas.drawPath(path, paint);
  }

  @override
  bool shouldRepaint(_RibbonPainter oldDelegate) => false;
}

// ---------------------------------------------------------------------------
// Purchase confirm bottom sheet
// ---------------------------------------------------------------------------

class _PurchaseConfirmSheet extends StatefulWidget {
  const _PurchaseConfirmSheet({
    required this.package,
    required this.ref,
  });

  final CoinPackageEntity package;
  final WidgetRef ref;

  @override
  State<_PurchaseConfirmSheet> createState() => _PurchaseConfirmSheetState();
}

class _PurchaseConfirmSheetState extends State<_PurchaseConfirmSheet> {
  bool _loading = false;

  Future<void> _purchase() async {
    setState(() => _loading = true);
    // Simulate payment processing delay.
    await Future<void>.delayed(const Duration(milliseconds: 1800));

    // In production: integrate Stripe SDK here using the clientSecret
    // returned by createPaymentIntent. For now, call confirmPurchase directly
    // with a simulated paymentIntentId.
    final success = await widget.ref
        .read(purchaseProvider.notifier)
        .buyPackage(widget.package.id, widget.ref);

    if (!mounted) return;
    Navigator.pop(context);

    if (success) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            'Added ${NumberFormat.compact().format(widget.package.totalCoins)} coins to your wallet',
          ),
          backgroundColor: const Color(0xFF4CAF50),
          behavior: SnackBarBehavior.floating,
        ),
      );
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Purchase failed. Please try again.'),
          backgroundColor: Color(0xFFFF2D55),
          behavior: SnackBarBehavior.floating,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final pkg = widget.package;
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 24, 20, 32),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Handle
          Container(
            width: 40,
            height: 4,
            decoration: BoxDecoration(
              color: const Color(0xFF3A3A3A),
              borderRadius: BorderRadius.circular(2),
            ),
          ),
          const SizedBox(height: 20),
          // Coin icon
          Container(
            width: 64,
            height: 64,
            decoration: const BoxDecoration(
              color: Color(0xFFFFD700),
              shape: BoxShape.circle,
            ),
            child: const Icon(Icons.monetization_on,
                color: Colors.white, size: 40),
          ),
          const SizedBox(height: 16),
          Text(
            '${NumberFormat.compact().format(pkg.coins)} Coins',
            style: const TextStyle(
              color: Colors.white,
              fontSize: 24,
              fontWeight: FontWeight.bold,
            ),
          ),
          if (pkg.bonusCoins > 0) ...[
            const SizedBox(height: 4),
            Text(
              '+${NumberFormat.compact().format(pkg.bonusCoins)} bonus coins included',
              style: const TextStyle(
                color: Color(0xFF4CAF50),
                fontSize: 13,
              ),
            ),
          ],
          const SizedBox(height: 8),
          Text(
            '\$${pkg.price.toStringAsFixed(2)} ${pkg.currency}',
            style: const TextStyle(
              color: Color(0xFF888888),
              fontSize: 16,
            ),
          ),
          const SizedBox(height: 28),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: _loading ? null : _purchase,
              style: ElevatedButton.styleFrom(
                backgroundColor: const Color(0xFFFF2D55),
                disabledBackgroundColor:
                    const Color(0xFFFF2D55).withValues(alpha: 0.5),
                padding: const EdgeInsets.symmetric(vertical: 16),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(12),
                ),
              ),
              child: _loading
                  ? const SizedBox(
                      width: 22,
                      height: 22,
                      child: CircularProgressIndicator(
                        strokeWidth: 2.5,
                        color: Colors.white,
                      ),
                    )
                  : Text(
                      'Buy for \$${pkg.price.toStringAsFixed(2)}',
                      style: const TextStyle(
                        color: Colors.white,
                        fontWeight: FontWeight.bold,
                        fontSize: 16,
                      ),
                    ),
            ),
          ),
          const SizedBox(height: 12),
          const Text(
            'Purchases are non-refundable. By continuing you agree to our Terms.',
            textAlign: TextAlign.center,
            style: TextStyle(color: Color(0xFF555555), fontSize: 11),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Empty state
// ---------------------------------------------------------------------------

class _EmptyPackages extends StatelessWidget {
  const _EmptyPackages();

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.monetization_on, color: Color(0xFF333333), size: 56),
          SizedBox(height: 16),
          Text(
            'No packages available',
            style: TextStyle(color: Color(0xFF888888), fontSize: 16),
          ),
          SizedBox(height: 6),
          Text(
            'Check back soon for coin offers.',
            style: TextStyle(color: Color(0xFF555555), fontSize: 13),
          ),
        ],
      ),
    );
  }
}

