import '../../domain/entities/wallet_entity.dart';

// ---------------------------------------------------------------------------
// WalletModel
// ---------------------------------------------------------------------------

class WalletModel extends WalletEntity {
  const WalletModel({
    required super.coinBalance,
    required super.diamondBalance,
    required super.updatedAt,
  });

  factory WalletModel.fromJson(Map<String, dynamic> json) {
    return WalletModel(
      coinBalance: (json['coin_balance'] as num?)?.toInt() ?? 0,
      diamondBalance: (json['diamond_balance'] as num?)?.toInt() ?? 0,
      updatedAt: json['updated_at'] != null
          ? DateTime.parse(json['updated_at'] as String)
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
        'coin_balance': coinBalance,
        'diamond_balance': diamondBalance,
        'updated_at': updatedAt.toIso8601String(),
      };
}

// ---------------------------------------------------------------------------
// CoinPackageModel
// ---------------------------------------------------------------------------

class CoinPackageModel extends CoinPackageEntity {
  const CoinPackageModel({
    required super.id,
    required super.coins,
    required super.bonusCoins,
    required super.price,
    required super.currency,
    required super.isBestValue,
  });

  factory CoinPackageModel.fromJson(Map<String, dynamic> json) {
    return CoinPackageModel(
      id: json['id'] as String,
      coins: (json['coins'] as num?)?.toInt() ?? 0,
      bonusCoins: (json['bonus_coins'] as num?)?.toInt() ?? 0,
      price: (json['price'] as num?)?.toDouble() ?? 0.0,
      currency: json['currency'] as String? ?? 'USD',
      isBestValue: json['is_best_value'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'coins': coins,
        'bonus_coins': bonusCoins,
        'price': price,
        'currency': currency,
        'is_best_value': isBestValue,
      };
}

// ---------------------------------------------------------------------------
// TransactionModel
// ---------------------------------------------------------------------------

class TransactionModel extends TransactionEntity {
  const TransactionModel({
    required super.id,
    required super.type,
    required super.amount,
    required super.currency,
    required super.referenceId,
    required super.description,
    required super.createdAt,
  });

  factory TransactionModel.fromJson(Map<String, dynamic> json) {
    return TransactionModel(
      id: json['id'] as String,
      type: _parseType(json['type'] as String? ?? 'earn'),
      amount: (json['amount'] as num?)?.toInt() ?? 0,
      currency: json['currency'] as String? ?? 'coins',
      referenceId: json['reference_id'] as String? ?? '',
      description: json['description'] as String? ?? '',
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'type': type.name,
        'amount': amount,
        'currency': currency,
        'reference_id': referenceId,
        'description': description,
        'created_at': createdAt.toIso8601String(),
      };

  static TransactionType _parseType(String s) {
    switch (s) {
      case 'buy':
        return TransactionType.buy;
      case 'gift':
        return TransactionType.gift;
      case 'tip':
        return TransactionType.tip;
      case 'withdraw':
        return TransactionType.withdraw;
      default:
        return TransactionType.earn;
    }
  }
}
