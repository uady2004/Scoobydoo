import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';

import '../../domain/entities/wallet_entity.dart';
import '../providers/wallet_provider.dart';
import 'buy_coins_screen.dart';

class WalletScreen extends ConsumerWidget {
  const WalletScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final walletAsync = ref.watch(walletProvider);
    final transactionsAsync = ref.watch(transactionsProvider);

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: const Text(
          'Balance',
          style: TextStyle(
            color: Colors.white,
            fontWeight: FontWeight.bold,
            fontSize: 18,
          ),
        ),
        centerTitle: true,
        elevation: 0,
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh, color: Colors.white),
            onPressed: () => ref.read(walletProvider.notifier).refresh(),
          ),
        ],
      ),
      body: RefreshIndicator(
        color: const Color(0xFFFF2D55),
        backgroundColor: const Color(0xFF1A1A1A),
        onRefresh: () async {
          ref.read(walletProvider.notifier).refresh();
          ref.read(transactionsProvider.notifier).refresh();
        },
        child: CustomScrollView(
          slivers: [
            SliverToBoxAdapter(
              child: walletAsync.when(
                data: (wallet) => _BalanceCard(wallet: wallet),
                loading: () => const _BalanceCardSkeleton(),
                error: (e, _) => _ErrorCard(message: e.toString()),
              ),
            ),
            const SliverToBoxAdapter(
              child: Padding(
                padding: EdgeInsets.fromLTRB(16, 24, 16, 8),
                child: Text(
                  'Transaction history',
                  style: TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.bold,
                    fontSize: 16,
                  ),
                ),
              ),
            ),
            transactionsAsync.when(
              data: (state) => state.items.isEmpty
                  ? const SliverFillRemaining(child: _EmptyTransactions())
                  : _TransactionList(state: state),
              loading: () => const SliverFillRemaining(
                child: Center(
                  child: CircularProgressIndicator(color: Color(0xFFFF2D55)),
                ),
              ),
              error: (e, _) => SliverFillRemaining(
                child: _ErrorCard(message: e.toString()),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Balance card
// ---------------------------------------------------------------------------

class _BalanceCard extends StatelessWidget {
  const _BalanceCard({required this.wallet});

  final WalletEntity wallet;

  @override
  Widget build(BuildContext context) {
    final usdValue = wallet.diamondBalance / 100.0;

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
      child: Container(
        decoration: BoxDecoration(
          gradient: const LinearGradient(
            colors: [Color(0xFFFF2D55), Color(0xFFFF6B8A)],
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
          ),
          borderRadius: BorderRadius.circular(16),
          boxShadow: [
            BoxShadow(
              color: const Color(0xFFFF2D55).withValues(alpha: 0.35),
              blurRadius: 20,
              offset: const Offset(0, 8),
            ),
          ],
        ),
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Coins row
            Row(
              children: [
                const _CoinIcon(),
                const SizedBox(width: 10),
                Text(
                  '${_fmt(wallet.coinBalance)} Coins',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 28,
                    fontWeight: FontWeight.bold,
                    letterSpacing: -0.5,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 14),
            // Diamonds row
            Row(
              children: [
                const _DiamondIcon(),
                const SizedBox(width: 10),
                Text(
                  '${_fmt(wallet.diamondBalance)} Diamonds',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 6),
            // USD equivalent
            Padding(
              padding: const EdgeInsets.only(left: 34),
              child: Text(
                '≈ \$${usdValue.toStringAsFixed(2)} USD',
                style: TextStyle(
                  color: Colors.white.withValues(alpha: 0.7),
                  fontSize: 12,
                ),
              ),
            ),
            const SizedBox(height: 20),
            // Action buttons
            Row(
              children: [
                Expanded(
                  child: _GradientButton(
                    label: 'Recharge',
                    icon: Icons.add_circle_outline,
                    onTap: () {
                      Navigator.push(
                        context,
                        MaterialPageRoute<void>(
                          builder: (_) => const BuyCoinsScreen(),
                        ),
                      );
                    },
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: _OutlinedButton(
                    label: 'Withdraw',
                    icon: Icons.account_balance_wallet_outlined,
                    onTap: () => _showWithdrawSheet(context),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showWithdrawSheet(BuildContext context) {
    showModalBottomSheet<void>(
      context: context,
      backgroundColor: const Color(0xFF1A1A1A),
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (_) => const _WithdrawSheet(),
    );
  }

  static String _fmt(int n) => NumberFormat.compact().format(n);
}

class _CoinIcon extends StatelessWidget {
  const _CoinIcon();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 28,
      height: 28,
      decoration: const BoxDecoration(
        color: Color(0xFFFFD700),
        shape: BoxShape.circle,
      ),
      child: const Icon(Icons.monetization_on, color: Colors.white, size: 18),
    );
  }
}

class _DiamondIcon extends StatelessWidget {
  const _DiamondIcon();

  @override
  Widget build(BuildContext context) {
    return const SizedBox(
      width: 24,
      height: 24,
      child: Icon(Icons.diamond, color: Color(0xFF00E5FF), size: 22),
    );
  }
}

class _GradientButton extends StatelessWidget {
  const _GradientButton({
    required this.label,
    required this.icon,
    required this.onTap,
  });

  final String label;
  final IconData icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 12),
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, color: const Color(0xFFFF2D55), size: 18),
            const SizedBox(width: 6),
            Text(
              label,
              style: const TextStyle(
                color: Color(0xFFFF2D55),
                fontWeight: FontWeight.bold,
                fontSize: 14,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _OutlinedButton extends StatelessWidget {
  const _OutlinedButton({
    required this.label,
    required this.icon,
    required this.onTap,
  });

  final String label;
  final IconData icon;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 12),
        decoration: BoxDecoration(
          border: Border.all(color: Colors.white.withValues(alpha: 0.6)),
          borderRadius: BorderRadius.circular(10),
        ),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, color: Colors.white, size: 18),
            const SizedBox(width: 6),
            Text(
              label,
              style: const TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.bold,
                fontSize: 14,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Transaction list
// ---------------------------------------------------------------------------

class _TransactionList extends ConsumerWidget {
  const _TransactionList({required this.state});

  final TransactionState state;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final grouped = _groupByDate(state.items);
    final sections = grouped.keys.toList();

    return SliverList(
      delegate: SliverChildBuilderDelegate(
        (context, index) {
          // Tally: each section = 1 header + N items.
          int runningIndex = 0;
          for (final label in sections) {
            final items = grouped[label]!;
            if (index == runningIndex) {
              return _DateHeader(label: label);
            }
            runningIndex++;
            final itemIndex = index - runningIndex;
            if (itemIndex < items.length) {
              final isLast = itemIndex == items.length - 1;
              return _TransactionTile(
                transaction: items[itemIndex],
                showDivider: !isLast,
              );
            }
            runningIndex += items.length;
          }
          // Load more trigger
          if (state.hasMore && !state.isLoading) {
            ref.read(transactionsProvider.notifier).loadMore();
            return const Padding(
              padding: EdgeInsets.all(16),
              child: Center(
                child: CircularProgressIndicator(color: Color(0xFFFF2D55)),
              ),
            );
          }
          return const SizedBox(height: 32);
        },
        childCount: _totalCount(grouped) + 1,
      ),
    );
  }

  static int _totalCount(Map<String, List<TransactionEntity>> grouped) {
    return grouped.values.fold(0, (sum, items) => sum + items.length + 1);
  }

  static Map<String, List<TransactionEntity>> _groupByDate(
    List<TransactionEntity> items,
  ) {
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final yesterday = today.subtract(const Duration(days: 1));
    final result = <String, List<TransactionEntity>>{};

    for (final tx in items) {
      final d = DateTime(tx.createdAt.year, tx.createdAt.month, tx.createdAt.day);
      String label;
      if (d == today) {
        label = 'Today';
      } else if (d == yesterday) {
        label = 'Yesterday';
      } else {
        label = DateFormat('MMM d, yyyy').format(tx.createdAt);
      }
      result.putIfAbsent(label, () => []).add(tx);
    }
    return result;
  }
}

class _DateHeader extends StatelessWidget {
  const _DateHeader({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 20, 16, 6),
      child: Text(
        label,
        style: const TextStyle(
          color: Color(0xFF888888),
          fontSize: 12,
          fontWeight: FontWeight.w600,
          letterSpacing: 0.5,
        ),
      ),
    );
  }
}

class _TransactionTile extends StatelessWidget {
  const _TransactionTile({
    required this.transaction,
    required this.showDivider,
  });

  final TransactionEntity transaction;
  final bool showDivider;

  @override
  Widget build(BuildContext context) {
    final isCredit = transaction.isCredit;
    final amountStr =
        '${isCredit ? '+' : '-'}${transaction.amount} ${transaction.currency}';

    return Column(
      children: [
        ListTile(
          contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
          leading: Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: _iconBg(transaction.type),
              shape: BoxShape.circle,
            ),
            child: Icon(_icon(transaction.type), color: Colors.white, size: 20),
          ),
          title: Text(
            transaction.description.isNotEmpty
                ? transaction.description
                : _defaultDescription(transaction.type),
            style: const TextStyle(
              color: Colors.white,
              fontSize: 14,
              fontWeight: FontWeight.w500,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
          subtitle: Text(
            DateFormat('h:mm a').format(transaction.createdAt),
            style: const TextStyle(color: Color(0xFF888888), fontSize: 12),
          ),
          trailing: Text(
            amountStr,
            style: TextStyle(
              color: isCredit
                  ? const Color(0xFF4CAF50)
                  : const Color(0xFFFF2D55),
              fontWeight: FontWeight.bold,
              fontSize: 14,
            ),
          ),
        ),
        if (showDivider)
          const Divider(
            height: 1,
            indent: 72,
            endIndent: 16,
            color: Color(0xFF2A2A2A),
          ),
      ],
    );
  }

  IconData _icon(TransactionType type) {
    switch (type) {
      case TransactionType.buy:
        return Icons.add_circle;
      case TransactionType.gift:
        return Icons.card_giftcard;
      case TransactionType.tip:
        return Icons.favorite;
      case TransactionType.earn:
        return Icons.stars;
      case TransactionType.withdraw:
        return Icons.account_balance_wallet;
    }
  }

  Color _iconBg(TransactionType type) {
    switch (type) {
      case TransactionType.buy:
        return const Color(0xFF4CAF50);
      case TransactionType.gift:
        return const Color(0xFFFF2D55);
      case TransactionType.tip:
        return const Color(0xFFE91E8C);
      case TransactionType.earn:
        return const Color(0xFFFFD700);
      case TransactionType.withdraw:
        return const Color(0xFF2196F3);
    }
  }

  String _defaultDescription(TransactionType type) {
    switch (type) {
      case TransactionType.buy:
        return 'Coin purchase';
      case TransactionType.gift:
        return 'Gift sent';
      case TransactionType.tip:
        return 'Tip sent';
      case TransactionType.earn:
        return 'Coins earned';
      case TransactionType.withdraw:
        return 'Withdrawal';
    }
  }
}

// ---------------------------------------------------------------------------
// Withdraw sheet
// ---------------------------------------------------------------------------

class _WithdrawSheet extends StatefulWidget {
  const _WithdrawSheet();

  @override
  State<_WithdrawSheet> createState() => _WithdrawSheetState();
}

class _WithdrawSheetState extends State<_WithdrawSheet> {
  final _controller = TextEditingController();
  String _method = 'PayPal';

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        16,
        20,
        16,
        MediaQuery.of(context).viewInsets.bottom + 20,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Withdraw Diamonds',
            style: TextStyle(
              color: Colors.white,
              fontSize: 18,
              fontWeight: FontWeight.bold,
            ),
          ),
          const SizedBox(height: 6),
          const Text(
            '100 diamonds = \$1.00 USD',
            style: TextStyle(color: Color(0xFF888888), fontSize: 13),
          ),
          const SizedBox(height: 20),
          TextField(
            controller: _controller,
            keyboardType: TextInputType.number,
            style: const TextStyle(color: Colors.white),
            decoration: InputDecoration(
              hintText: 'Amount in diamonds',
              hintStyle: const TextStyle(color: Color(0xFF555555)),
              filled: true,
              fillColor: const Color(0xFF2A2A2A),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(10),
                borderSide: BorderSide.none,
              ),
              prefixIcon: const Icon(Icons.diamond, color: Color(0xFF00E5FF)),
            ),
          ),
          const SizedBox(height: 16),
          const Text(
            'Payout method',
            style: TextStyle(color: Color(0xFF888888), fontSize: 13),
          ),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            children: ['PayPal', 'Bank transfer', 'Crypto']
                .map(
                  (m) => ChoiceChip(
                    label: Text(m),
                    selected: _method == m,
                    onSelected: (_) => setState(() => _method = m),
                    selectedColor: const Color(0xFFFF2D55),
                    backgroundColor: const Color(0xFF2A2A2A),
                    labelStyle: TextStyle(
                      color: _method == m ? Colors.white : const Color(0xFF888888),
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                )
                .toList(),
          ),
          const SizedBox(height: 20),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: () {
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(
                    content: Text('Withdrawal request submitted'),
                    backgroundColor: Color(0xFF4CAF50),
                  ),
                );
              },
              style: ElevatedButton.styleFrom(
                backgroundColor: const Color(0xFFFF2D55),
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10),
                ),
              ),
              child: const Text(
                'Submit withdrawal',
                style: TextStyle(
                  color: Colors.white,
                  fontWeight: FontWeight.bold,
                  fontSize: 16,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Skeleton / empty / error states
// ---------------------------------------------------------------------------

class _BalanceCardSkeleton extends StatelessWidget {
  const _BalanceCardSkeleton();

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
      child: Container(
        height: 180,
        decoration: BoxDecoration(
          color: const Color(0xFF1A1A1A),
          borderRadius: BorderRadius.circular(16),
        ),
        child: const Center(
          child: CircularProgressIndicator(color: Color(0xFFFF2D55)),
        ),
      ),
    );
  }
}

class _EmptyTransactions extends StatelessWidget {
  const _EmptyTransactions();

  @override
  Widget build(BuildContext context) {
    return const Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.receipt_long, color: Color(0xFF333333), size: 56),
          SizedBox(height: 16),
          Text(
            'No transactions yet',
            style: TextStyle(color: Color(0xFF888888), fontSize: 16),
          ),
          SizedBox(height: 6),
          Text(
            'Buy coins or send gifts to get started.',
            style: TextStyle(color: Color(0xFF555555), fontSize: 13),
          ),
        ],
      ),
    );
  }
}

class _ErrorCard extends StatelessWidget {
  const _ErrorCard({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Text(
          message,
          style: const TextStyle(color: Color(0xFF888888), fontSize: 14),
          textAlign: TextAlign.center,
        ),
      ),
    );
  }
}
